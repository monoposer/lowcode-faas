# Build from repository root so paths stay consistent with CI.
# docker build -f playground/Dockerfile .
FROM node:22-alpine AS build
WORKDIR /app
COPY playground/package.json playground/package-lock.json ./
RUN npm ci
COPY playground/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY playground/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
