# Reuse the current primary image's verified Python/Chromium runtime only when
# dependencies are identical. Application sources come entirely from Git.
ARG BASE_IMAGE
FROM ${BASE_IMAGE}
ARG COMMIT
COPY payload/backend/requirements.txt /tmp/okad-requirements.txt
RUN cmp /tmp/okad-requirements.txt /app/backend/requirements.txt
RUN rm -rf /app/backend/app /app/frontend-v2
COPY payload/backend/ /app/backend/
COPY payload/frontend-v2/ /app/frontend-v2/
LABEL org.opencontainers.image.revision=${COMMIT}
