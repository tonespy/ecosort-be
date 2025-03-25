# Build stage
FROM golang:1.23

# Install wget and unzip if needed
RUN apt-get update && apt-get install -y wget unzip

# Download and extract the TensorFlow C library.
# Adjust the version and URL as needed.
RUN wget https://storage.googleapis.com/tensorflow/versions/2.18.1/libtensorflow-cpu-linux-x86_64.tar.gz && \
    tar -C /usr/local -xzf libtensorflow-cpu-linux-x86_64.tar.gz && \
    rm libtensorflow-cpu-linux-x86_64.tar.gz

# Set the CGO flags so that Go can locate the TensorFlow headers and library.
ENV CGO_CFLAGS="-I/usr/local/include"
ENV CGO_LDFLAGS="-L/usr/local/lib -ltensorflow"

WORKDIR /app
COPY . .
RUN go build -o main ./cmd/server/

EXPOSE 8080
CMD ["./main"]