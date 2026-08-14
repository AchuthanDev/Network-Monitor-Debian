FROM node:22.17.0-alpine AS build
WORKDIR /src/apps/frontend
COPY apps/frontend/package.json apps/frontend/package-lock.json ./
RUN npm ci
COPY apps/frontend ./
RUN npm run build

FROM nginx:1.29.0-alpine
COPY --from=build /src/apps/frontend/dist /usr/share/nginx/html
COPY deployments/docker/nginx/default.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
