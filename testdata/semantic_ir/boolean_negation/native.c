#include <stdbool.h>
extern bool sec_fn_0(bool);       // Not
extern bool sec_fn_1(bool);       // Double
extern bool sec_fn_3(void);       // Once
extern bool sec_fn_4(bool, bool); // EqualNegated
extern bool sec_fn_5(bool);       // Choice
extern bool sec_fn_6(bool);       // Stored
int main(void) {
    for (int value = 0; value < 2; ++value) {
        if (sec_fn_0(value) != !value) return 1;
        if (sec_fn_1(value) != value) return 2;
        if (sec_fn_5(value) != !value) return 3;
        if (sec_fn_6(value) != value) return 4;
    }
    for (int left = 0; left < 2; ++left)
        for (int right = 0; right < 2; ++right)
            if (sec_fn_4(left, right) != (left == right)) return 5;
    if (!sec_fn_3()) return 6;
    return 0;
}
