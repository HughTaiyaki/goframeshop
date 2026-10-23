# syntax=docker/dockerfile:1.7
# Elasticsearch with the IK Chinese analyzer baked in, instead of copying the
# plugin into a running container by hand: rebuilding gives the same result
# every time, and the checksum below makes the build fail if the download
# ever changes. The plugin version must match Elasticsearch exactly.
FROM docker.elastic.co/elasticsearch/elasticsearch:8.11.0

ADD --chown=elasticsearch:elasticsearch \
    --checksum=sha256:ae32133c324a72379b9fafa5006c77f4c07a6269d22df8cc6528a29d09054b00 \
    https://release.infinilabs.com/analysis-ik/stable/elasticsearch-analysis-ik-8.11.0.zip \
    /tmp/analysis-ik.zip

RUN bin/elasticsearch-plugin install --batch file:///tmp/analysis-ik.zip \
 && rm /tmp/analysis-ik.zip
