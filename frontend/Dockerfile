FROM node:24-alpine AS development-dependencies-env
COPY . /app
WORKDIR /app
RUN npm ci

FROM node:24-alpine AS production-dependencies-env
COPY ./package.json package-lock.json /app/
WORKDIR /app
RUN npm ci --omit=dev

FROM node:24-alpine AS build-env
COPY . /app/
COPY --from=development-dependencies-env /app/node_modules /app/node_modules
WORKDIR /app
# VITE_* values are inlined into the client bundle at build time, not read
# at runtime — the defaults baked into app/lib/* (localhost:8080/8081) are
# exactly right for this repo's docker-compose demo, so no build args are
# needed here. Override them (as --build-arg + ARG/ENV above) only when
# deploying somewhere the API/Keycloak aren't reachable at those addresses.
RUN npm run build

FROM node:24-alpine
COPY ./package.json package-lock.json /app/
COPY --from=production-dependencies-env /app/node_modules /app/node_modules
COPY --from=build-env /app/build /app/build
WORKDIR /app
EXPOSE 3000
CMD ["npm", "run", "start"]
