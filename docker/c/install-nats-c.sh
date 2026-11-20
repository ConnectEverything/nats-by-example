set -euo pipefail

# Pinned so builds are reproducible and don't depend on the (rate-limited)
# GitHub API. Bump deliberately and verify with `nbe verify c`.
rel="${NATS_C_VERSION:-v3.15.0}"
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
