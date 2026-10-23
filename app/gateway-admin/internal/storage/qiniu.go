package storage

import (
	"bytes"
	"context"
	"os"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/qiniu/go-sdk/v7/auth"
	qiniu "github.com/qiniu/go-sdk/v7/storage"
)

// QiniuConfig is the qiniu config section. AccessKey and SecretKey may come
// from QINIU_ACCESS_KEY / QINIU_SECRET_KEY instead, so they never sit in a
// file. Never commit them, and never use keys copied from a tutorial: those
// belong to someone else's account.
type QiniuConfig struct {
	AccessKey string
	SecretKey string
	Bucket    string
	// Domain is the bucket's download domain, e.g. https://img.example.com.
	Domain string
	// Region is a Qiniu region id: z0 East China, z1 North, z2 South, ...
	Region string
	// MaxBytes and MimeLimit are written into the upload token, so Qiniu
	// itself refuses anything larger or of another type.
	MaxBytes  int64
	MimeLimit string
}

// Qiniu stores objects in a Qiniu Kodo bucket.
type Qiniu struct {
	cfg        QiniuConfig
	cred       *auth.Credentials
	uploader   *qiniu.FormUploader
	bucketMgr  *qiniu.BucketManager
	baseDomain string
}

func NewQiniu(cfg QiniuConfig) (*Qiniu, error) {
	if cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" || cfg.Domain == "" {
		return nil, gerror.New("qiniu needs accessKey, secretKey, bucket and domain")
	}
	region, ok := qiniu.GetRegionByID(qiniu.RegionID(cfg.Region))
	if !ok {
		return nil, gerror.Newf("unknown qiniu.region %q", cfg.Region)
	}
	uploadCfg := &qiniu.Config{Region: &region, UseHTTPS: true}
	cred := auth.New(cfg.AccessKey, cfg.SecretKey)
	return &Qiniu{
		cfg:        cfg,
		cred:       cred,
		uploader:   qiniu.NewFormUploader(uploadCfg),
		bucketMgr:  qiniu.NewBucketManager(cred, uploadCfg),
		baseDomain: strings.TrimRight(cfg.Domain, "/"),
	}, nil
}

func NewQiniuFromConfig(ctx context.Context) (*Qiniu, error) {
	var cfg QiniuConfig
	section, err := g.Cfg().Get(ctx, "qiniu")
	if err != nil {
		return nil, gerror.Wrap(err, "read qiniu config")
	}
	if err := section.Scan(&cfg); err != nil {
		return nil, gerror.Wrap(err, "parse qiniu config")
	}
	if cfg.AccessKey == "" {
		cfg.AccessKey = os.Getenv("QINIU_ACCESS_KEY")
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = os.Getenv("QINIU_SECRET_KEY")
	}
	if cfg.Region == "" {
		cfg.Region = "z2"
	}
	return NewQiniu(cfg)
}

func (q *Qiniu) Name() string { return "qiniu" }

func (q *Qiniu) Put(ctx context.Context, key string, data []byte, _ string) (string, error) {
	// The token is scoped to this one key with insertOnly, expires in ten
	// minutes, and carries the size and type limits: even if it leaked, it
	// could only create this single object.
	policy := qiniu.PutPolicy{
		Scope:      q.cfg.Bucket + ":" + key,
		InsertOnly: 1,
		Expires:    600,
		FsizeLimit: q.cfg.MaxBytes,
		MimeLimit:  q.cfg.MimeLimit,
	}
	var ret qiniu.PutRet
	err := q.uploader.Put(ctx, &ret, policy.UploadToken(q.cred), key, bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		// The SDK error can contain request details; callers log only that
		// the upload failed.
		return "", gerror.Wrap(err, "qiniu put")
	}
	return q.baseDomain + "/" + key, nil
}

func (q *Qiniu) Delete(_ context.Context, key string) error {
	if err := q.bucketMgr.Delete(q.cfg.Bucket, key); err != nil {
		return gerror.Wrap(err, "qiniu delete")
	}
	return nil
}
