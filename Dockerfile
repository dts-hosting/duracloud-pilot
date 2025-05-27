FROM public.ecr.aws/docker/library/golang:1.24 AS build-image
ARG FUNCTION_NAME
WORKDIR /src
COPY . .
WORKDIR /src/cmd/$FUNCTION_NAME
RUN go build -o /src/lambda-handler

FROM public.ecr.aws/lambda/provided:al2023
COPY --from=build-image /src/lambda-handler .
ENTRYPOINT ["./lambda-handler"]
