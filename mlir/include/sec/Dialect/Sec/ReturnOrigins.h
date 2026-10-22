#ifndef SEC_DIALECT_SEC_RETURNORIGINS_H
#define SEC_DIALECT_SEC_RETURNORIGINS_H

#include "llvm/ADT/SmallVector.h"
#include "llvm/ADT/StringRef.h"
#include <cstdint>
#include <optional>

namespace sec {

// Canonical `sec.return_origins` entries (MD-084 § 4):
//
//   origin     := root projection*
//   root       := "param#" unsigned | "receiver" | "static@" unsigned
//   projection := ".field#" unsigned | "[" unsigned "]" | "[*]"
//   unsigned   := "0" | nonzero-digit digit*
//
// Entries are machine-parsable, case-sensitive and whitespace-free; anything
// else is rejected rather than normalized. Type-aware admissibility of the
// projections is the verifier's job (MD-085).
// Rules: rules/mlir/sec_mlir_dialect.md — "Reference metadata (MD-084)".
struct ReturnOriginProjection {
  enum class Kind { Field, Index, AnyIndex };
  Kind kind;
  uint32_t value = 0;

  bool operator==(const ReturnOriginProjection &other) const {
    return kind == other.kind && value == other.value;
  }
};

struct ReturnOrigin {
  enum class Root { Parameter, Receiver, Static };
  Root root;
  uint32_t rootIndex = 0;
  llvm::SmallVector<ReturnOriginProjection, 4> projections;
};

namespace detail {
// consumeCanonicalUnsigned accepts "0" or a digit string without a leading
// zero that fits in 32 bits.
inline std::optional<uint32_t> consumeCanonicalUnsigned(llvm::StringRef &text) {
  llvm::StringRef digits =
      text.take_while([](char c) { return c >= '0' && c <= '9'; });
  if (digits.empty() || (digits.size() > 1 && digits.front() == '0'))
    return std::nullopt;
  uint64_t value = 0;
  if (digits.getAsInteger(10, value) || value > UINT32_MAX)
    return std::nullopt;
  text = text.drop_front(digits.size());
  return static_cast<uint32_t>(value);
}
} // namespace detail

inline std::optional<ReturnOrigin> parseReturnOrigin(llvm::StringRef text) {
  ReturnOrigin origin;
  if (text.consume_front("param#")) {
    std::optional<uint32_t> index = detail::consumeCanonicalUnsigned(text);
    if (!index)
      return std::nullopt;
    origin.root = ReturnOrigin::Root::Parameter;
    origin.rootIndex = *index;
  } else if (text.consume_front("static@")) {
    std::optional<uint32_t> index = detail::consumeCanonicalUnsigned(text);
    if (!index)
      return std::nullopt;
    origin.root = ReturnOrigin::Root::Static;
    origin.rootIndex = *index;
  } else if (text.consume_front("receiver")) {
    origin.root = ReturnOrigin::Root::Receiver;
  } else {
    return std::nullopt;
  }
  while (!text.empty()) {
    if (text.consume_front(".field#")) {
      std::optional<uint32_t> field = detail::consumeCanonicalUnsigned(text);
      if (!field)
        return std::nullopt;
      origin.projections.push_back({ReturnOriginProjection::Kind::Field, *field});
    } else if (text.consume_front("[*]")) {
      origin.projections.push_back({ReturnOriginProjection::Kind::AnyIndex, 0});
    } else if (text.consume_front("[")) {
      std::optional<uint32_t> index = detail::consumeCanonicalUnsigned(text);
      if (!index || !text.consume_front("]"))
        return std::nullopt;
      origin.projections.push_back({ReturnOriginProjection::Kind::Index, *index});
    } else {
      return std::nullopt;
    }
  }
  return origin;
}

} // namespace sec

#endif // SEC_DIALECT_SEC_RETURNORIGINS_H
