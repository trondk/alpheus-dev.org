# The component as an image: one static binary on scratch, running as uid
# 65532 as the pod requires. Build it from your component's own module
# (see README.md). Pin the base image by digest for a component you run;
# the StreamRun pins the component's image by digest (GO-26).
FROM docker.io/library/golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /component .

FROM scratch
COPY --from=build /component /component
USER 65532:65532
ENTRYPOINT ["/component"]
