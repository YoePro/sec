#include "llvm/ADT/APInt.h"
#include "llvm/ADT/ArrayRef.h"
#include "llvm/Support/raw_ostream.h"

#include <array>
#include <cstdint>
#include <cstdlib>
#include <random>
#include <vector>

using llvm::APInt;

namespace {

[[noreturn]] void fail(unsigned width, const char *operation) {
  llvm::errs() << "Package 8 APInt differential mismatch for " << operation
               << " at width " << width << '\n';
  std::exit(1);
}

void expect(bool condition, unsigned width, const char *operation) {
  if (!condition)
    fail(width, operation);
}

std::vector<APInt> values(unsigned width, std::mt19937_64 &random) {
  std::vector<APInt> result{
      APInt(width, 0),
      APInt(width, 1),
      APInt::getMaxValue(width),
      APInt::getSignedMinValue(width),
      APInt::getSignedMaxValue(width),
  };
  unsigned words = APInt::getNumWords(width);
  for (unsigned sample = 0; sample != 24; ++sample) {
    std::vector<uint64_t> bits(words);
    for (uint64_t &word : bits)
      word = random();
    result.emplace_back(width, llvm::ArrayRef<uint64_t>(bits));
  }
  return result;
}

void checkArithmetic(unsigned width, const std::vector<APInt> &samples) {
  APInt signedMinimum = APInt::getSignedMinValue(width);
  APInt signedMaximum = APInt::getSignedMaxValue(width);
  APInt unsignedMaximum = APInt::getMaxValue(width);
  unsigned addWidth = width + 1;
  unsigned multiplyWidth = width * 2;

  for (const APInt &left : samples) {
    bool referenceOverflow = false;
    APInt reference = -left;
    bool loweredOverflow = left == signedMinimum;
    expect(reference == (-left) && loweredOverflow == (left == signedMinimum),
           width, "negate");

    for (const APInt &right : samples) {
      APInt signedLeft = left.sext(addWidth);
      APInt signedRight = right.sext(addWidth);
      APInt unsignedLeft = left.zext(addWidth);
      APInt unsignedRight = right.zext(addWidth);

      APInt lowered = signedLeft + signedRight;
      bool loweredFailed = lowered.slt(signedMinimum.sext(addWidth)) ||
                           lowered.sgt(signedMaximum.sext(addWidth));
      reference = left.sadd_ov(right, referenceOverflow);
      expect(lowered.trunc(width) == reference &&
                 loweredFailed == referenceOverflow,
             width, "signed add");

      lowered = unsignedLeft + unsignedRight;
      loweredFailed = lowered.ugt(unsignedMaximum.zext(addWidth));
      reference = left.uadd_ov(right, referenceOverflow);
      expect(lowered.trunc(width) == reference &&
                 loweredFailed == referenceOverflow,
             width, "unsigned add");

      lowered = signedLeft - signedRight;
      loweredFailed = lowered.slt(signedMinimum.sext(addWidth)) ||
                      lowered.sgt(signedMaximum.sext(addWidth));
      reference = left.ssub_ov(right, referenceOverflow);
      expect(lowered.trunc(width) == reference &&
                 loweredFailed == referenceOverflow,
             width, "signed subtract");

      reference = left.usub_ov(right, referenceOverflow);
      expect((left - right) == reference &&
                 left.ult(right) == referenceOverflow,
             width, "unsigned subtract");

      APInt signedProduct = left.sext(multiplyWidth) * right.sext(multiplyWidth);
      loweredFailed = signedProduct.slt(signedMinimum.sext(multiplyWidth)) ||
                      signedProduct.sgt(signedMaximum.sext(multiplyWidth));
      reference = left.smul_ov(right, referenceOverflow);
      expect(signedProduct.trunc(width) == reference &&
                 loweredFailed == referenceOverflow,
             width, "signed multiply");

      APInt unsignedProduct =
          left.zext(multiplyWidth) * right.zext(multiplyWidth);
      loweredFailed = unsignedProduct.ugt(unsignedMaximum.zext(multiplyWidth));
      reference = left.umul_ov(right, referenceOverflow);
      expect(unsignedProduct.trunc(width) == reference &&
                 loweredFailed == referenceOverflow,
             width, "unsigned multiply");

      bool signedInvalid = right.isZero() ||
                           (left == signedMinimum && right.isAllOnes());
      APInt safeRight = signedInvalid ? APInt(width, 1) : right;
      APInt signedQuotient = left.sdiv(safeRight);
      APInt signedRemainder = left.srem(safeRight);
      expect(signedInvalid || signedQuotient == left.sdiv(right), width,
             "signed divide");
      expect(signedInvalid || signedRemainder == left.srem(right), width,
             "signed remainder");

      bool unsignedInvalid = right.isZero();
      safeRight = unsignedInvalid ? APInt(width, 1) : right;
      expect(unsignedInvalid || left.udiv(safeRight) == left.udiv(right), width,
             "unsigned divide");
      expect(unsignedInvalid || left.urem(safeRight) == left.urem(right), width,
             "unsigned remainder");

      bool loweredSignedLess = left.sext(addWidth).slt(right.sext(addWidth));
      bool loweredUnsignedLess = left.zext(addWidth).ult(right.zext(addWidth));
      expect(loweredSignedLess == left.slt(right) &&
                 loweredUnsignedLess == left.ult(right),
             width, "comparisons");
    }
  }
}

void checkShifts(unsigned width, const std::vector<APInt> &samples) {
  std::array<unsigned, 5> counts{0, 1, width / 2, width - 1, width};
  for (const APInt &value : samples) {
    for (unsigned count : counts) {
      bool invalid = count >= width;
      unsigned safeCount = invalid ? 0 : count;
      bool referenceOverflow = false;
      APInt signedReference = value.sshl_ov(safeCount, referenceOverflow);
      APInt wide = value.sext(width * 2).shl(safeCount);
      bool signedFailed = invalid ||
                          wide.slt(APInt::getSignedMinValue(width).sext(width * 2)) ||
                          wide.sgt(APInt::getSignedMaxValue(width).sext(width * 2));
      expect(wide.trunc(width) == signedReference &&
                 signedFailed == (invalid || referenceOverflow),
             width, "signed left shift");
      expect(invalid || value.shl(safeCount) == value.shl(count), width,
             "unsigned left shift");
      expect(invalid || value.ashr(safeCount) == value.ashr(count), width,
             "signed right shift");
      expect(invalid || value.lshr(safeCount) == value.lshr(count), width,
             "unsigned right shift");
    }
  }
}

} // namespace

int main() {
  std::mt19937_64 random(0x5345435041434b38ULL);
  for (unsigned width : {8U, 16U, 32U, 64U, 128U, 256U}) {
    std::vector<APInt> samples = values(width, random);
    checkArithmetic(width, samples);
    checkShifts(width, samples);
  }
  return 0;
}
