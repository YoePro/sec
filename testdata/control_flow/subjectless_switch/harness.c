#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

static int32_t events[16];
static int count;

int32_t probe(int32_t tag, int32_t answer) {
    if (count >= 16) abort();
    events[count++] = tag;
    return answer;
}

void touch(int32_t tag) {
    if (count >= 16) abort();
    events[count++] = tag;
}

extern int32_t Alternatives(int32_t, int32_t, int32_t);
extern int32_t Logical(int32_t, int32_t, int32_t);
extern int32_t NoDefault(int32_t);
extern int32_t EmptyBody(int32_t, int32_t);
extern int32_t Nested(int32_t, int32_t);
extern int32_t LoopControl(void);
extern int32_t DefaultOnly(void);
extern int32_t EmptySwitch(void);

static void check(const char *name, int32_t result, int32_t expected,
                  const int32_t *trace, int length) {
    if (result != expected || count != length) {
        fprintf(stderr, "%s: result %d (expected %d), %d events (expected %d)\n",
                name, result, expected, count, length);
        exit(1);
    }
    for (int i = 0; i < length; i++) {
        if (events[i] != trace[i]) {
            fprintf(stderr, "%s: event %d = %d, expected %d\n", name, i, events[i], trace[i]);
            exit(1);
        }
    }
}

// Every trace includes the selected body, catches duplicate condition reads,
// and forbids evaluation of both skipped RHS calls and later else-if clauses.
#define CHECK(name, call, expected, ...) do { \
    const int32_t trace[] = { __VA_ARGS__ }; \
    count = 0; \
    check(name, call, expected, trace, sizeof(trace) / sizeof(trace[0])); \
} while (0)

int32_t run_tests(void) {
    CHECK("first alternative", Alternatives(1, 1, 1), 10, 1, 10);
    CHECK("second alternative", Alternatives(0, 1, 1), 10, 1, 2, 10);
    CHECK("next clause", Alternatives(0, 0, 1), 20, 1, 2, 3, 20);
    CHECK("default", Alternatives(0, 0, 0), 30, 1, 2, 3, 30);
    CHECK("logical skip RHS", Logical(0, 1, 1), 40, 1, 3, 10);
    CHECK("logical skip alternative", Logical(1, 1, 1), 40, 1, 2, 10);
    CHECK("logical false", Logical(1, 0, 0), 40, 1, 2, 3, 20);
    CHECK("unmatched continuation", NoDefault(0), 40, 1);
    CHECK("matched continuation", NoDefault(1), 40, 1, 10);
    CHECK("empty body stops", EmptyBody(1, 1), 40, 1, 40);
    CHECK("after empty body", EmptyBody(0, 1), 40, 1, 2, 20, 40);
    CHECK("nested first", Nested(1, 1), 40, 1, 2, 10);
    CHECK("nested default", Nested(1, 0), 40, 1, 2, 20);
    CHECK("nested skipped", Nested(0, 1), 40, 1, 30);
    CHECK("enclosing loop control", LoopControl(), 2, 20);
    CHECK("only default", DefaultOnly(), 40, 10);
    count = 0;
    check("empty switch", EmptySwitch(), 40, NULL, 0);
    puts("17 subjectless switch scenarios passed");
    return 0;
}
