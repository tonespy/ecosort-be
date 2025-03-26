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

# Optionally set the LD_LIBRARY_PATH to ensure the runtime linker finds the library.
ENV LD_LIBRARY_PATH="/usr/local/lib:${LD_LIBRARY_PATH}"
ENV LIBRARY_PATH="${LIBRARY_PATH}:/usr/local/lib"
ENV TF_CPP_MIN_LOG_LEVEL=2
ENV TF_ENABLE_ONEDNN_OPTS=0

WORKDIR /app
COPY . .
RUN go build -o ./tmp/main ./cmd/server/

EXPOSE 8080
CMD ["./tmp/main"]