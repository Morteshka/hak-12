FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install --no-audit --no-fund
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
# файлы из архива с Windows бывают с правами 0700 — nginx их не прочтёт (403 на /version.txt, /samples)
RUN chmod -R a+rX /usr/share/nginx/html
EXPOSE 80
