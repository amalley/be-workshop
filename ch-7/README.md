# Docker
## Build
Run `docker build -t wikistats-ch2:latest .` from. ch-2 directory

### Specify Golang Build Image 
You can override the build image by adding the build arg GO_IMAGE to the build command.

`docker build --build-arg GO_IMAGE=1.26.1-alpine -t wikistats-ch2:latest .`

## Run
Run `docker run -p 7000:7000 wikistats-ch2:latest` once built.

### Providing port and stream URL
`docker run -p 8080:8080 wikistats-ch2:latest --port 8080`

`docker run -p 7000:7000 wikistats-ch2:latest --url http://some.other.url.com`