import os

import lit.formats
from lit.llvm import llvm_config

config.name = "SEC_MLIR"
config.test_format = lit.formats.ShTest()
config.suffixes = [".mlir"]
config.test_source_root = os.path.dirname(__file__)
config.test_exec_root = os.path.join(config.sec_mlir_obj_root, "test")
config.excludes = ["CMakeLists.txt", "lit.cfg.py", "lit.site.cfg.py.in"]

llvm_config.use_default_substitutions()
llvm_config.with_system_environment(["HOME", "INCLUDE", "LIB", "TMP", "TEMP"])
llvm_config.with_environment("PATH", config.llvm_tools_dir, append_path=True)

tool_dirs = [os.path.join(config.sec_mlir_obj_root, "bin"), config.llvm_tools_dir]
llvm_config.add_tool_substitutions(["sec-mlir-opt", "FileCheck", "not"], tool_dirs)
