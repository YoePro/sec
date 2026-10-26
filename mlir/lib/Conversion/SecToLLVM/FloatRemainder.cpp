#include "FloatLowering.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/IR/PatternMatch.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "llvm/ADT/APFloat.h"

using namespace mlir;

// Binary long division computes |x| mod |y| exactly using integer
// significands. Exponents decrease on every reduction step, bounding the loop
// even at the largest finite / smallest subnormal pair. Normalization and
// encoding retain gradual underflow; the final bit pattern takes x's sign.
// Zero divisors, infinite x and NaNs give NaN; finite x modulo infinity is x.
// No potentially invalid FP-to-int cast or external fmod call is introduced.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§6–9,23,75–80;
// rules/foundations/operators.md — "Floating remainder".
void sec::lowerFloatRemainder(FloatBinaryOp op, IRRewriter &b) {
  Location loc = op.getLoc();
  Value x = op.getLeft(), y = op.getRight();
  auto type = cast<FloatType>(x.getType());
  unsigned width = type.getWidth(), fractionBits = width == 32 ? 23 : 52;
  auto integer = b.getIntegerType(width);
  Block *entry = op->getBlock();
  Block *continuation = b.splitBlock(entry, op->getIterator());
  Value result = continuation->addArgument(type, loc);
  auto block = [&](unsigned arguments) {
    return b.createBlock(continuation, SmallVector<Type>(arguments, integer),
                          SmallVector<Location>(arguments, loc));
  };
  Block *invalid = block(0), *small = block(0), *zeroResult = block(0);
  Block *compare = block(0), *prepare = block(0);
  Block *normX = block(2), *normXStep = block(2);
  Block *normY = block(4), *normYStep = block(4);
  Block *divide = block(4), *divideStep = block(4), *finish = block(3);
  Block *normalize = block(2), *normalizeStep = block(2), *encode = block(2);
  b.setInsertionPointToEnd(entry);
  auto constant = [&](const APInt &value) -> Value {
    return arith::ConstantOp::create(b, loc, integer, b.getIntegerAttr(integer, value));
  };
  auto number = [&](int64_t value) { return constant(APInt(width, value, true)); };
  Value z = number(0), one = number(1), shift = number(fractionBits);
  APInt signMask = APInt::getSignMask(width);
  APInt fractionMask = APInt::getLowBitsSet(width, fractionBits);
  APInt exponentMask = ~(signMask | fractionMask);
  Value hidden = constant(APInt::getOneBitSet(width, fractionBits));
  Value fraction = constant(fractionMask), exponent = constant(exponentMask);
  Value xb = arith::BitcastOp::create(b, loc, integer, x);
  Value yb = arith::BitcastOp::create(b, loc, integer, y);
  Value sign = arith::AndIOp::create(b, loc, xb, constant(signMask));
  Value ax = arith::AndIOp::create(b, loc, xb, constant(~signMask));
  Value ay = arith::AndIOp::create(b, loc, yb, constant(~signMask));
  auto cmp = [&](arith::CmpIPredicate predicate, Value a, Value c) -> Value {
    return arith::CmpIOp::create(b, loc, predicate, a, c);
  };
  // Absolute bit order is unsigned numeric order for nonnegative IEEE values.
  Value badX = cmp(arith::CmpIPredicate::uge, ax, exponent);
  Value nanY = cmp(arith::CmpIPredicate::ugt, ay, exponent);
  Value zeroY = cmp(arith::CmpIPredicate::eq, ay, z);
  Value badY = arith::OrIOp::create(b, loc, nanY, zeroY);
  Value bad = arith::OrIOp::create(b, loc, badX, badY);
  cf::CondBranchOp::create(b, loc, bad, invalid, ValueRange{}, compare, ValueRange{});

  b.setInsertionPointToEnd(invalid);
  Value nan = arith::ConstantOp::create(b, loc, type,
      b.getFloatAttr(type, APFloat::getQNaN(type.getFloatSemantics())));
  cf::BranchOp::create(b, loc, continuation, ValueRange{nan});
  b.setInsertionPointToEnd(compare);
  Value less = cmp(arith::CmpIPredicate::ult, ax, ay);
  cf::CondBranchOp::create(b, loc, less, small, ValueRange{}, prepare, ValueRange{});
  b.setInsertionPointToEnd(small);
  cf::BranchOp::create(b, loc, continuation, ValueRange{x});
  b.setInsertionPointToEnd(zeroResult);
  Value signedZero = arith::BitcastOp::create(b, loc, type, sign);
  cf::BranchOp::create(b, loc, continuation, ValueRange{signedZero});

  b.setInsertionPointToEnd(prepare);
  Value ex = arith::ShRUIOp::create(b, loc, ax, shift);
  Value ey = arith::ShRUIOp::create(b, loc, ay, shift);
  Value sx = arith::AndIOp::create(b, loc, ax, fraction);
  Value sy = arith::AndIOp::create(b, loc, ay, fraction);
  Value xNormal = cmp(arith::CmpIPredicate::ne, ex, z);
  Value yNormal = cmp(arith::CmpIPredicate::ne, ey, z);
  sx = arith::SelectOp::create(b, loc, xNormal,
      arith::OrIOp::create(b, loc, sx, hidden), sx);
  sy = arith::SelectOp::create(b, loc, yNormal,
      arith::OrIOp::create(b, loc, sy, hidden), sy);
  ex = arith::SelectOp::create(b, loc, xNormal, ex, one);
  ey = arith::SelectOp::create(b, loc, yNormal, ey, one);
  cf::BranchOp::create(b, loc, normX, ValueRange{sx, ex});

  b.setInsertionPointToEnd(normX);
  Value nx = normX->getArgument(0), xe = normX->getArgument(1);
  Value xReady = cmp(arith::CmpIPredicate::uge, nx, hidden);
  cf::CondBranchOp::create(b, loc, xReady, normY, ValueRange{sy, ey, nx, xe},
                           normXStep, ValueRange{nx, xe});
  b.setInsertionPointToEnd(normXStep);
  cf::BranchOp::create(b, loc, normX, ValueRange{
      arith::ShLIOp::create(b, loc, normXStep->getArgument(0), one),
      arith::SubIOp::create(b, loc, normXStep->getArgument(1), one)});

  b.setInsertionPointToEnd(normY);
  Value ny = normY->getArgument(0), ye = normY->getArgument(1);
  Value yReady = cmp(arith::CmpIPredicate::uge, ny, hidden);
  cf::CondBranchOp::create(b, loc, yReady, divide,
      ValueRange{normY->getArgument(2), normY->getArgument(3), ny, ye},
      normYStep, normY->getArguments());
  b.setInsertionPointToEnd(normYStep);
  cf::BranchOp::create(b, loc, normY, ValueRange{
      arith::ShLIOp::create(b, loc, normYStep->getArgument(0), one),
      arith::SubIOp::create(b, loc, normYStep->getArgument(1), one),
      normYStep->getArgument(2), normYStep->getArgument(3)});

  b.setInsertionPointToEnd(divide);
  Value more = cmp(arith::CmpIPredicate::sgt, divide->getArgument(1), divide->getArgument(3));
  cf::CondBranchOp::create(b, loc, more, divideStep, divide->getArguments(),
      finish, ValueRange{divide->getArgument(0), divide->getArgument(2), divide->getArgument(3)});
  b.setInsertionPointToEnd(divideStep);
  Value dx = divideStep->getArgument(0), dy = divideStep->getArgument(2);
  Value subtract = cmp(arith::CmpIPredicate::uge, dx, dy);
  Value remainder = arith::SelectOp::create(b, loc, subtract,
      arith::SubIOp::create(b, loc, dx, dy), dx);
  cf::BranchOp::create(b, loc, divide, ValueRange{
      arith::ShLIOp::create(b, loc, remainder, one),
      arith::SubIOp::create(b, loc, divideStep->getArgument(1), one),
      dy, divideStep->getArgument(3)});

  b.setInsertionPointToEnd(finish);
  Value fx = finish->getArgument(0), fy = finish->getArgument(1);
  Value reduce = cmp(arith::CmpIPredicate::uge, fx, fy);
  Value rest = arith::SelectOp::create(b, loc, reduce,
      arith::SubIOp::create(b, loc, fx, fy), fx);
  Value isZero = cmp(arith::CmpIPredicate::eq, rest, z);
  cf::CondBranchOp::create(b, loc, isZero, zeroResult, ValueRange{},
      normalize, ValueRange{rest, finish->getArgument(2)});
  b.setInsertionPointToEnd(normalize);
  Value ready = cmp(arith::CmpIPredicate::uge, normalize->getArgument(0), hidden);
  cf::CondBranchOp::create(b, loc, ready, encode, normalize->getArguments(),
      normalizeStep, normalize->getArguments());
  b.setInsertionPointToEnd(normalizeStep);
  cf::BranchOp::create(b, loc, normalize, ValueRange{
      arith::ShLIOp::create(b, loc, normalizeStep->getArgument(0), one),
      arith::SubIOp::create(b, loc, normalizeStep->getArgument(1), one)});

  b.setInsertionPointToEnd(encode);
  Value significand = encode->getArgument(0), finalExponent = encode->getArgument(1);
  Value subnormal = cmp(arith::CmpIPredicate::sle, finalExponent, z);
  Value safeShift = arith::SelectOp::create(b, loc, subnormal,
      arith::SubIOp::create(b, loc, one, finalExponent), z);
  Value subBits = arith::ShRUIOp::create(b, loc, significand, safeShift);
  Value normalBits = arith::OrIOp::create(b, loc,
      arith::SubIOp::create(b, loc, significand, hidden),
      arith::ShLIOp::create(b, loc, finalExponent, shift));
  Value magnitude = arith::SelectOp::create(b, loc, subnormal, subBits, normalBits);
  Value signedBits = arith::OrIOp::create(b, loc, sign, magnitude);
  Value answer = arith::BitcastOp::create(b, loc, type, signedBits);
  cf::BranchOp::create(b, loc, continuation, ValueRange{answer});
  b.replaceOp(op, result);
}
