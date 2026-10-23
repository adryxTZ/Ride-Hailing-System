# Backup/restore toolbox: pg_dump/pg_restore (Postgres 16 client, matching the managed
# server's major version), age (encryption) and rclone (S3-compatible upload).
# Built on the server by compose (profile "ops"); holds scripts only, no data.
FROM postgres:16-alpine
RUN apk add --no-cache age rclone bash \
 # rclone is configured only by RCLONE_CONFIG_STORE_* env vars; an empty file silences its notice.
 && mkdir -p /root/.config/rclone && touch /root/.config/rclone/rclone.conf
COPY backup.sh restore.sh /usr/local/bin/
COPY entry.sh /usr/local/bin/entry
ENTRYPOINT ["/usr/local/bin/entry"]
CMD ["backup"]
