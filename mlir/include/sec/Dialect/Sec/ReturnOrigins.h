#ifndef SEC_DIALECT_SEC_RETURNORIGINS_H
#define SEC_DIALECT_SEC_RETURNORIGINS_H

#include "llvm/ADT/SmallVector.h"
#include "llvm/ADT/StringRef.h"
#include <cstdint>
#include <optional>
#include <string>
#include <algorithm>
#include "llvm/ADT/APInt.h"
#include "llvm/ADT/SmallString.h"

namespace sec {

// Canonical `sec.return_origins` entries (MD-084 § 4):
//
//   origin     := root projection*
//   root       := "param#" unsigned | "receiver" | "static@" unsigned
//   projection := ".field#" unsigned | "[" unsigned "]" | "[*]"
// Schema 12 (MD-115) also admits "[" natural "..<" natural "]" | "[?]";
// range endpoints are arbitrary precision, unlike legacy uint32 tokens.
//   unsigned   := "0" | nonzero-digit digit*
//
// Entries are machine-parsable, case-sensitive and whitespace-free; anything
// else is rejected rather than normalized. Type-aware admissibility of the
// projections is the verifier's job (MD-085).
// Rules: rules/mlir/sec_mlir_dialect.md — "Reference metadata (MD-084)", v12.34.
struct ReturnOriginProjection {
  enum class Kind { Field, Index, AnyIndex, Range, UnknownRange };
  Kind kind;
  uint32_t value = 0;
  std::string start, end;

  bool operator==(const ReturnOriginProjection &other) const {
    return kind == other.kind && value == other.value && start == other.start && end == other.end;
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
// MD-115 §§2,6: retain arbitrary-precision canonical decimal endpoints.
inline std::optional<std::string> consumeNatural(llvm::StringRef &text) {
  auto digits = text.take_while([](char c) { return c >= '0' && c <= '9'; });
  if (digits.empty() || (digits.size() > 1 && digits.front() == '0')) return std::nullopt;
  text = text.drop_front(digits.size());
  return digits.str();
}
inline bool naturalLess(llvm::StringRef a, llvm::StringRef b) {
  return a.size() != b.size() ? a.size() < b.size() : a < b;
}
inline std::string addNatural(llvm::StringRef a, llvm::StringRef b) {
  unsigned width = std::max(a.size(), b.size()) * 4 + 2;
  llvm::APInt x(width, a, 10), y(width, b, 10);
  llvm::SmallString<40> result;
  (x+y).toString(result, 10, false);
  return result.str().str();
}
inline std::optional<std::string> subtractNatural(llvm::StringRef a, llvm::StringRef b) {
  if (naturalLess(a,b)) return std::nullopt;
  unsigned width = std::max(a.size(),b.size())*4+2;
  llvm::APInt x(width,a,10), y(width,b,10); llvm::SmallString<40> text;
  (x-y).toString(text,10,false); return text.str().str();
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
    } else if (text.consume_front("[?]")) {
      origin.projections.push_back({ReturnOriginProjection::Kind::UnknownRange, 0});
    } else if (text.starts_with("[") && text.contains("..<")) {
      // A range must close before any following projection.
      auto close = text.find(']');
      auto marker = text.find("..<");
      if (close != llvm::StringRef::npos && marker < close) {
        text = text.drop_front();
        auto start = detail::consumeNatural(text);
        if (!start || !text.consume_front("..<")) return std::nullopt;
        auto end = detail::consumeNatural(text);
        if (!end || !text.consume_front("]") || detail::naturalLess(*end,*start)) return std::nullopt;
        origin.projections.push_back({ReturnOriginProjection::Kind::Range, 0, *start, *end});
      } else {
        text = text.drop_front();
        auto index = detail::consumeCanonicalUnsigned(text);
        if (!index || !text.consume_front("]")) return std::nullopt;
        origin.projections.push_back({ReturnOriginProjection::Kind::Index, *index});
      }
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

// MD-115 §4: compose source-relative exact intervals; dynamic containment
// retains an unknown range. All decimal arithmetic is arbitrary precision.
inline std::optional<ReturnOrigin> normalizeReturnOrigin(ReturnOrigin origin) {
  llvm::SmallVector<ReturnOriginProjection,4> path;
  for (auto step : origin.projections) {
    bool range = step.kind == ReturnOriginProjection::Kind::Range || step.kind == ReturnOriginProjection::Kind::UnknownRange;
    if (range && !path.empty() && (path.back().kind == ReturnOriginProjection::Kind::Range || path.back().kind == ReturnOriginProjection::Kind::UnknownRange)) {
      auto prior = path.pop_back_val();
      if (prior.kind == ReturnOriginProjection::Kind::UnknownRange || step.kind == ReturnOriginProjection::Kind::UnknownRange) {
        step = {ReturnOriginProjection::Kind::UnknownRange,0};
      } else {
        // Check relative containment without truncating either endpoint.
        if (detail::naturalLess(prior.end, detail::addNatural(prior.start,step.end))) return std::nullopt;
        step.start = detail::addNatural(prior.start,step.start);
        step.end = detail::addNatural(prior.start,step.end);
      }
    }
    path.push_back(step);
  }
  origin.projections = std::move(path);
  return origin;
}

} // namespace sec

#endif // SEC_DIALECT_SEC_RETURNORIGINS_H
