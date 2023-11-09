FROM redis

COPY redis.conf /etc/redis/redis.conf
# Create a non-root user and group
RUN adduser --disabled-password --gecos "" sab

# Set the appropriate permissions for Redis data directory
RUN chown -R sab:sab /data

# Switch to the non-root user
USER sab

# Start the Redis server
CMD ["redis-server", "/etc/redis/redis.conf"]
