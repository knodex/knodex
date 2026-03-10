# Build stage
FROM node:22-alpine AS builder

WORKDIR /app

# Build argument for enterprise mode
ARG BUILD_MODE=""

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build the application (enterprise or OSS based on BUILD_MODE)
RUN if [ "$BUILD_MODE" = "enterprise" ]; then \
        npm run build:enterprise; \
    else \
        npm run build; \
    fi

# Runtime stage - nginx for static serving
FROM nginx:alpine AS runner

# Copy custom nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy built application
COPY --from=builder /app/dist /usr/share/nginx/html

# Create non-root user
RUN addgroup -g 1001 -S appgroup && \
    adduser -S appuser -u 1001 -G appgroup && \
    chown -R appuser:appgroup /usr/share/nginx/html && \
    chown -R appuser:appgroup /var/cache/nginx && \
    chown -R appuser:appgroup /var/log/nginx && \
    touch /var/run/nginx.pid && \
    chown -R appuser:appgroup /var/run/nginx.pid

USER appuser

EXPOSE 3000

CMD ["nginx", "-g", "daemon off;"]
