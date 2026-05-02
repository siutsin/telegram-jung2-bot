# Copyright (c) Meta Platforms, Inc. and affiliates.
#
# This source code is dual-licensed under either the MIT license found in the
# LICENSE-MIT file in the root directory of this source tree or the Apache
# License, Version 2.0 found in the LICENSE-APACHE file in the root directory
# of this source tree. You may select, at your option, one of the
# above-listed licenses.

# TODO: Migrate back to @prelude//toolchains/go:system_go_toolchain.bzl once
# upstream exposes the race setting on system_go_toolchain.

load("@prelude//go:toolchain.bzl", "GoToolchainInfo")
load("@prelude//os_lookup:defs.bzl", "ScriptLanguage")
load("@prelude//utils:cmd_script.bzl", "cmd_script")

def go_platform() -> (str, str):
    arch = host_info().arch
    if arch.is_aarch64:
        go_arch = "arm64"
    elif arch.is_x86_64:
        go_arch = "amd64"
    else:
        fail("Unsupported go arch: {}".format(arch))

    os = host_info().os
    if os.is_macos:
        go_os = "darwin"
    elif os.is_linux:
        go_os = "linux"
    elif os.is_windows:
        go_os = "windows"
    else:
        fail("Unsupported go os: {}".format(os))

    return go_os, go_arch

def _configurable_system_go_toolchain_impl(ctx):
    go_os, go_arch = go_platform()

    script_language = ScriptLanguage("bat" if go_os == "windows" else "sh")
    go = "go.exe" if go_os == "windows" else "go"

    go_root = ctx.actions.declare_output("goroot", dir = True, has_content_based_path = True)

    # We need a physical GOROOT artifact so Buck can project individual source files.
    ctx.actions.run([ctx.attrs.copy_goroot[RunInfo], "-o", go_root.as_output()], category = "go_copy_goroot")

    suffix = ".exe" if go_os == "windows" else ""
    tool_prefix = "pkg/tool/{}_{}".format(go_os, go_arch)

    return [
        DefaultInfo(sub_targets = {"go": [
            RunInfo(go),
        ]}),
        GoToolchainInfo(
            asan = ctx.attrs.asan,
            assembler = RunInfo(cmd_script(ctx.actions, "asm", cmd_args(go, "tool", "asm"), script_language)),
            assembler_flags = [],
            build_tags = [],
            cgo = RunInfo(go_root.project(tool_prefix + "/cgo" + suffix)),
            compiler = RunInfo(cmd_script(ctx.actions, "compile", cmd_args(go, "tool", "compile"), script_language)),
            compiler_flags = [],
            cover = RunInfo(cmd_script(ctx.actions, "cover", cmd_args(go, "tool", "cover"), script_language)),
            cxx_compiler_flags = [],
            env_go_arch = go_arch,
            env_go_os = go_os,
            env_go_root = go_root,
            external_linker_flags = [],
            fuzz = False,
            gen_embedcfg = ctx.attrs.gen_embedcfg[RunInfo],
            go = RunInfo(cmd_script(ctx.actions, "go", cmd_args(go), script_language)),
            go_wrapper = ctx.attrs.go_wrapper[RunInfo],
            linker = RunInfo(cmd_script(ctx.actions, "link", cmd_args(go, "tool", "link"), script_language)),
            linker_flags = [],
            packer = RunInfo(cmd_script(ctx.actions, "pack", cmd_args(go, "tool", "pack"), script_language)),
            pkg_analyzer = ctx.attrs.pkg_analyzer[RunInfo],
            race = ctx.attrs.race,
            version = None,
        ),
    ]

configurable_system_go_toolchain = rule(
    impl = _configurable_system_go_toolchain_impl,
    attrs = {
        "asan": attrs.bool(default = False),
        "copy_goroot": attrs.default_only(attrs.dep(providers = [RunInfo], default = "prelude//go/tools:copy_goroot")),
        "gen_embedcfg": attrs.default_only(attrs.dep(providers = [RunInfo], default = "prelude//go/tools:gen_embedcfg")),
        "go_wrapper": attrs.default_only(attrs.dep(providers = [RunInfo], default = "prelude//go/tools:go_wrapper")),
        "pkg_analyzer": attrs.default_only(attrs.dep(providers = [RunInfo], default = "prelude//go/tools:pkg_analyzer")),
        "race": attrs.bool(default = False),
    },
    is_toolchain_rule = True,
)
