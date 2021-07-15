### Stage 1 - the build process
# Base image
FROM us-docker.pkg.dev/jonjohnson-test/sstp-hackathon/node:latest as builder

# set working directory
WORKDIR /app

# add `/app/node_modules/.bin` to $PATH
ENV PATH /app/node_modules/.bin:$PATH

# install and cache app dependencies
COPY package.json /app/package.json
RUN npm install --production --silent
COPY . ./

# build app
RUN npm run build


### Stage 2 - the production environment
FROM us-docker.pkg.dev/jonjohnson-test/sstp-hackathon/nginx:latest

# Copy built frontend to be served by nginx
COPY --from=builder /app/build /usr/share/nginx/html/

# Echo time into app file
RUN echo time

# Start nginx server
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
