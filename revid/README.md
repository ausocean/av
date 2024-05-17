# Readme

Package revid provides an API for a media capture/processing/forwarding
pipeline. The API is exposed in revid.go.

Configuration is handled by the config package.

Pipeline setup is handled in pipeline.go, where components of the pipeline are
pulled mostly from internal packages i.e. lexers, filters, packetisation and
protocols for forwarding.

Sending is handled by "senders", defined in the senders.go file.

Building the revid package requires gocv.io/x/gocv to be installed. On platforms
where this is not available, a reduced functionality package can be built using
the `withcv` build tag.
