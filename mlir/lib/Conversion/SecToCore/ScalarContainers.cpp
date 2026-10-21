#include "ScalarContainers.h"

#include "mlir/Transforms/DialectConversion.h"
#include "sec/Dialect/Sec/SecAttributes.h"
#include "sec/Dialect/Sec/SecTypes.h"

using namespace mlir;

// addScalarContainerConversions resolves scalar leaves recursively while
// retaining semantic container identity, authority and exact array extents.
// The converter owns these callbacks; capture the context pointer by value.
// Rules: rules/mlir/packages/sec-mlir-dialect_package6.md sections 22-23;
// rules/mlir/packages/sec-mlir-dialect_package14.md section 102;
// rules/mlir/lowering-versions/sec_mlir_lowering_v11.md section 19;
// rules/mlir/packages/sec-mlir-dialect_package16.md section 112.
void sec::addScalarContainerConversions(TypeConverter &typeConverter,
                                        MLIRContext *context) {
  typeConverter.addConversion(
      [&typeConverter, context](sec::NamedType type) -> std::optional<Type> {
        Type base = typeConverter.convertType(type.getBase());
        if (!base)
          return std::nullopt;
        return sec::NamedType::get(context, type.getIdentity(), base);
      });
  typeConverter.addConversion(
      [&typeConverter, context](sec::DistinctType type) -> std::optional<Type> {
        Type base = typeConverter.convertType(type.getBase());
        if (!base)
          return std::nullopt;
        return sec::DistinctType::get(context, type.getIdentity(), base);
      });
  // SEC-MLIR Package 14 section 102 and P14-61 require Package 6 scalar
  // resolution to recurse through fixed arrays without changing their exact
  // length or introducing a physical array representation.
  typeConverter.addConversion(
      [&typeConverter, context](sec::ArrayType type) -> std::optional<Type> {
        Type element = typeConverter.convertType(type.getElementType());
        if (!element)
          return std::nullopt;
        if (element == type.getElementType())
          return type;
        return sec::ArrayType::get(context, element, type.getLength());
      });
  // rules/mlir/packages/sec-mlir-dialect_package14.md, section 51:
  // Result[U, IndexError] must retain exactly the converted component types
  // used by the existing Result construction and return flow.
  typeConverter.addConversion(
      [&typeConverter, context](sec::ResultType type) -> std::optional<Type> {
        Type success = typeConverter.convertType(type.getSuccessType());
        Type error = typeConverter.convertType(type.getErrorType());
        if (!success || !error)
          return std::nullopt;
        return sec::ResultType::get(context, success, error);
      });
  typeConverter.addConversion(
      [&typeConverter, context](sec::EnumType type) -> std::optional<Type> {
        Type underlying = typeConverter.convertType(type.getUnderlying());
        if (!underlying)
          return std::nullopt;
        if (underlying == type.getUnderlying())
          return type;
        return sec::EnumType::get(
            context, type.getIdentity(), underlying,
            type.getRepresentation(), type.getBitWidth(), type.getCases());
      });
  typeConverter.addConversion(
      [&typeConverter, context](sec::UnionType type) -> std::optional<Type> {
        SmallVector<Attribute> arguments;
        SmallVector<Attribute> variants;
        bool changed = false;
        for (Attribute attribute : type.getTypeArguments()) {
          auto typeAttribute = dyn_cast<TypeAttr>(attribute);
          if (!typeAttribute)
            return std::nullopt;
          Type converted = typeConverter.convertType(typeAttribute.getValue());
          if (!converted)
            return std::nullopt;
          changed |= converted != typeAttribute.getValue();
          arguments.push_back(TypeAttr::get(converted));
        }
        for (Attribute attribute : type.getVariants()) {
          auto variant = dyn_cast<sec::UnionVariantAttr>(attribute);
          if (!variant)
            return std::nullopt;
          Type payload = variant.getPayload();
          if (payload) {
            Type converted = typeConverter.convertType(payload);
            if (!converted)
              return std::nullopt;
            changed |= converted != payload;
            payload = converted;
          }
          SmallVector<Attribute> fields;
          for (Attribute fieldAttribute : variant.getFields()) {
            auto field = dyn_cast<sec::UnionFieldAttr>(fieldAttribute);
            if (!field)
              return std::nullopt;
            Type converted = typeConverter.convertType(field.getType());
            if (!converted)
              return std::nullopt;
            changed |= converted != field.getType();
            fields.push_back(sec::UnionFieldAttr::get(
                context, field.getName(), converted));
          }
          variants.push_back(sec::UnionVariantAttr::get(
              context, variant.getIndex(), variant.getName(),
              variant.getKind(), payload, ArrayAttr::get(context, fields)));
        }
        if (!changed)
          return type;
        return sec::UnionType::get(
            context, type.getIdentity(), ArrayAttr::get(context, arguments),
            ArrayAttr::get(context, variants));
      });
  typeConverter.addConversion(
      [&typeConverter, context](sec::StructType type) -> std::optional<Type> {
        SmallVector<Attribute> arguments;
        SmallVector<Attribute> fields;
        bool changed = false;
        for (Attribute attribute : type.getTypeArguments()) {
          auto typeAttribute = dyn_cast<TypeAttr>(attribute);
          if (!typeAttribute)
            return std::nullopt;
          Type converted = typeConverter.convertType(typeAttribute.getValue());
          if (!converted)
            return std::nullopt;
          changed |= converted != typeAttribute.getValue();
          arguments.push_back(TypeAttr::get(converted));
        }
        for (Attribute attribute : type.getFields()) {
          auto field = dyn_cast<sec::StructFieldAttr>(attribute);
          if (!field)
            return std::nullopt;
          Type converted = typeConverter.convertType(field.getType());
          if (!converted)
            return std::nullopt;
          changed |= converted != field.getType();
          fields.push_back(sec::StructFieldAttr::get(
              context, field.getOrdinal(), field.getName(), converted,
              field.getTags()));
        }
        if (!changed)
          return type;
        return sec::StructType::get(
            context, type.getIdentity(), ArrayAttr::get(context, arguments),
            ArrayAttr::get(context, fields));
      });
  // SEC-MLIR Package 15 section 32 and lowering v11 section 19: scalar
  // resolution recurses through referent types and keeps the Place and
  // reference wrappers, their authority and their metadata.
  typeConverter.addConversion(
      [&typeConverter, context](sec::PlaceType type) -> std::optional<Type> {
        Type element = typeConverter.convertType(type.getElementType());
        if (!element)
          return std::nullopt;
        if (element == type.getElementType())
          return type;
        return sec::PlaceType::get(context, element, type.getAuthority());
      });
  typeConverter.addConversion(
      [&typeConverter, context](sec::RefType type) -> std::optional<Type> {
        Type element = typeConverter.convertType(type.getElementType());
        if (!element)
          return std::nullopt;
        if (element == type.getElementType())
          return type;
        return sec::RefType::get(context, element);
      });
  typeConverter.addConversion(
      [&typeConverter, context](sec::RefMutType type) -> std::optional<Type> {
        Type element = typeConverter.convertType(type.getElementType());
        if (!element)
          return std::nullopt;
        if (element == type.getElementType())
          return type;
        return sec::RefMutType::get(context, element);
      });

  // P16 section 112: resolve scalar elements, preserving the bounded safe
  // reference wrapper and its shared/mutable authority. This is not descriptor
  // lowering; reference metadata remains attached to the existing values.
  typeConverter.addConversion(
      [&typeConverter, context](sec::SliceType type) -> std::optional<Type> {
        Type element = typeConverter.convertType(type.getElementType());
        if (!element)
          return std::nullopt;
        return sec::SliceType::get(context, element);
      });
  typeConverter.addConversion(
      [&typeConverter, context](sec::SliceMutType type) -> std::optional<Type> {
        Type element = typeConverter.convertType(type.getElementType());
        if (!element)
          return std::nullopt;
        return sec::SliceMutType::get(context, element);
      });
}
