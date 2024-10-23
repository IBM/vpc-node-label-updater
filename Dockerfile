FROM gcr.io/distroless/static:latest-amd64

# Default values
ARG git_commit_id=unknown
ARG git_remote_url=unknown
ARG build_date=unknown
ARG jenkins_build_number=unknown
ARG REPO_SOURCE_URL=blank
ARG BUILD_URL=blank

# Add Labels to image to show build details
LABEL git-commit-id=${git_commit_id}
LABEL git-remote-url=${git_remote_url}
LABEL build-date=${build_date}

# RUN microdnf update && microdnf install -y ca-certificates

#RUN mkdir -p /home/vpc-node-label-updater/
COPY ibm-csi-init-container /
ADD ibm-csi-init-container /vpc-node-label-updater
#RUN chmod +x /vpc-node-label-updater

USER 2121:2121

ENTRYPOINT ["/ibm-csi-init-container"]
