#include "sec/Dialect/Sec/SecDialect.h"
#include "sec/Dialect/Sec/SecTypes.h"

using namespace mlir;
using namespace sec;

#include "sec/Dialect/Sec/SecDialect.cpp.inc"

void SecDialect::initialize() { registerTypes(); }
