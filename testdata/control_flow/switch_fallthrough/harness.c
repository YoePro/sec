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

extern int32_t Subject(int32_t);
extern int32_t Subjectless(int32_t, int32_t);
extern int32_t EmptyFallthrough(int32_t);
extern int32_t Returning(int32_t);
extern int32_t Nested(int32_t);
extern int32_t SiblingScopes(int32_t);

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
    CHECK("subject chain bypasses tests", Subject(1), 0, 1, 10, 20, 30, 50);
    CHECK("subject second body", Subject(2), 0, 1, 2, 20, 30, 50);
    CHECK("subject third body", Subject(3), 0, 1, 2, 3, 30, 50);
    CHECK("subject default", Subject(4), 0, 1, 2, 3, 40, 50);
    CHECK("subjectless bypass", Subjectless(1, 0), 0, 1, 10, 20, 30, 40);
    CHECK("subjectless second", Subjectless(0, 1), 0, 1, 2, 20, 30, 40);
    CHECK("subjectless default", Subjectless(0, 0), 0, 1, 2, 30, 40);
    CHECK("empty fallthrough", EmptyFallthrough(1), 0, 1, 20);
    CHECK("empty fallthrough default", EmptyFallthrough(0), 0, 1, 20);
    CHECK("return after transfer", Returning(1), 30, 1, 10, 20);
    CHECK("direct return", Returning(0), 30, 1, 20);
    CHECK("nested successors", Nested(1), 0, 1, 2, 10, 20, 30, 40);
    CHECK("nested skipped", Nested(0), 0, 1, 40);
    CHECK("sibling scopes transfer", SiblingScopes(1), 30, 1, 10, 20);
    CHECK("sibling scope default", SiblingScopes(0), 20, 1, 20);
    puts("15 switch fallthrough scenarios passed");
    return 0;
}
