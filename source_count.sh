#!/bin/bash

# Säkerställ att vi exkluderar vendor- och dolda mappar (som .git)
EXCLUDE_PATHS="-not -path '*/vendor/*' -not -path '*/.*'"

echo "=================================================="
echo "              ROW COUNT PER CATEGORY           "
echo "=================================================="

# 1. DOCUMENTATION (MD/YAML)
echo -e "\n[1] Documentation (MD, YAML):"
eval "find . -type f \( -name '*.md' -o -name '*.yaml' -o -name '*.yml' \) $EXCLUDE_PATHS" | xargs cat 2>/dev/null | wc -l
    
# 2. SEC Lines (SEC)
echo -e "\n[2] Rows of sec (SEC):"
eval "find . -type f -name '*.sec' $EXCLUDE_PATHS" | xargs cat 2>/dev/null | wc -l

# 3. GO, C, C++ & COMPILER-FILES (GO/C/C++/MLIR/LLVM)
echo -e "\n[3] Program code & Architecture (GO, C, C++, MLIR, LLVM):"
eval "find . -type f \( -name '*.go' -o -name '*.c' -o -name '*.cpp' -o -name '*.h' -o -name '*.hpp' -o -name '*.mlir' -o -name '*.ll' -o -name '*.td' \) $EXCLUDE_PATHS" | xargs cat 2>/dev/null | wc -l

echo "=================================================="
