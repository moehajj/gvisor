load("//tools:defs.bzl", "go_library", "go_test")

package(licenses = ["notice"])

go_library(
    name = "base",
    testonly = 1,
    srcs = [
        "base.go",
    ],
    deps = ["//test/benchmarks/harness"],
)

go_test(
    name = "base_test",
    size = "small",
    srcs = [
        "size_test.go",
        "startup_test.go",
    ],
    library = ":base",
    tags = [
        # Requires docker and runsc to be configured before test runs.
        "manual",
        "local",
    ],
    deps = [
        "//pkg/test/dockerutil",
        "//test/benchmarks/harness",
        "//test/benchmarks/tools",
    ],
)
