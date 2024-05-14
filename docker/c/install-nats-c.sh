rel=$(curl -s https://api.github.com/repos/nats-io/nats.c/releases/latest | jq -r '.tag_name')
wget https://github.com/nats-io/nats.c/archive/refs/tags/${rel}.tar.gz
tar -xzf ${rel}.tar.gz
cd nats.c-${rel#v}
mkdir build
cd build
cmake \
    -DNATS_BUILD_TLS_USE_OPENSSL_1_1_API=ON \
    -DNATS_BUILD_STREAMING=OFF \
    -DNATS_BUILD_EXAMPLES=OFF \
    ..
make
make install


