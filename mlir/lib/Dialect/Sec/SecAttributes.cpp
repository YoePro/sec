#include "sec/Dialect/Sec/SecAttributes.h"

#include "sec/Dialect/Sec/SecDialect.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/StringSet.h"
#include "llvm/ADT/TypeSwitch.h"
#include "mlir/IR/Builders.h"
#include "mlir/IR/DialectImplementation.h"

using namespace mlir;
using namespace sec;

#define GET_ATTRDEF_CLASSES
#include "sec/Dialect/Sec/SecAttributes.cpp.inc"

LogicalResult EnumCaseAttr::verify(
    function_ref<InFlightDiagnostic()> emitError, uint32_t ordinal,
    StringAttr name, StringAttr value) {
  if (!name || name.getValue().empty())
    return emitError() << "enum case name must not be empty";
  if (!value || value.getValue().empty())
    return emitError() << "enum case value must be a base-10 integer string";
  StringRef spelling = value.getValue();
  spelling.consume_front("-");
  spelling.consume_front("+");
  if (spelling.empty() || !llvm::all_of(spelling, [](char c) {
        return c >= '0' && c <= '9';
      }))
    return emitError() << "enum case value must be a base-10 integer string";
  return success();
}

LogicalResult UnionFieldAttr::verify(
    function_ref<InFlightDiagnostic()> emitError, StringAttr name, Type type) {
  if (!name || name.getValue().empty())
    return emitError() << "union field name must not be empty";
  if (!type || isa<NoneType>(type))
    return emitError() << "union field type must be non-void";
  return success();
}

LogicalResult UnionVariantAttr::verify(
    function_ref<InFlightDiagnostic()> emitError, uint32_t index,
    StringAttr name, StringAttr kind, Type payload,
    ArrayAttr fields) {
  if (!name || name.getValue().empty())
    return emitError() << "union variant name must not be empty";
  if (!kind)
    return emitError() << "union variant kind must be present";
  if (kind.getValue() == "empty") {
    if (payload || !fields.empty())
      return emitError() << "empty union variant must not have payload";
  } else if (kind.getValue() == "single") {
    if (!payload || isa<NoneType>(payload) || !fields.empty())
      return emitError() << "single union variant requires one payload type";
  } else if (kind.getValue() == "fields") {
    if (payload || fields.empty())
      return emitError() << "fields union variant requires named fields only";
    llvm::StringSet<> names;
    for (Attribute attribute : fields) {
      auto field = dyn_cast<UnionFieldAttr>(attribute);
      if (!field)
        return emitError() << "union fields must be #sec.union_field attributes";
      if (!names.insert(field.getName().getValue()).second)
        return emitError() << "union payload field names must be unique";
    }
  } else {
    return emitError() << "union variant kind must be empty, single, or fields";
  }
  return success();
}

LogicalResult StructTagAttr::verify(
    function_ref<InFlightDiagnostic()> emitError, StringAttr key,
    StringAttr value) {
  if (!key || key.getValue().empty())
    return emitError() << "struct tag key must not be empty";
  if (!value)
    return emitError() << "struct tag value must be present";
  return success();
}

LogicalResult StructFieldAttr::verify(
    function_ref<InFlightDiagnostic()> emitError, uint32_t ordinal,
    StringAttr name, Type type, ArrayAttr tags) {
  if (!name || name.getValue().empty())
    return emitError() << "struct field name must not be empty";
  if (!type || isa<NoneType>(type))
    return emitError() << "struct field type must be non-void";
  for (Attribute attribute : tags)
    if (!isa<StructTagAttr>(attribute))
      return emitError() << "struct field tags must be #sec.struct_tag attributes";
  return success();
}

void SecDialect::registerAttributes() {
  addAttributes<
#define GET_ATTRDEF_LIST
#include "sec/Dialect/Sec/SecAttributes.cpp.inc"
      >();
}
