FROM rancher/dind:v0.2.0
COPY ./scripts/bootstrap /scripts/bootstrap
RUN /scripts/bootstrap
