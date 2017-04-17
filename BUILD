load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_prefix")

go_prefix("github.com/rockstardevs/parser")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    visibility = ["//visibility:private"],
    deps = [
        "//ofx:go_default_library",
        "@com_github_golang_glog//:go_default_library",
    ],
)

go_binary(
    name = "parser",
    library = ":go_default_library",
    visibility = ["//visibility:public"],
)
