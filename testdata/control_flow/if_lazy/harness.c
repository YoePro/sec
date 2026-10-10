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

extern int32_t Chain(int32_t, int32_t, int32_t);
extern int32_t And(int32_t, int32_t);
extern int32_t Or(int32_t, int32_t);
extern int32_t Nested(int32_t, int32_t, int32_t);
extern int32_t LogicalChain(int32_t, int32_t, int32_t, int32_t, int32_t);

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
    CHECK("chain first", Chain(1, 1, 1), 10, 1, 10);
    CHECK("chain second", Chain(0, 1, 1), 20, 1, 2, 20);
    CHECK("chain third", Chain(0, 0, 1), 30, 1, 2, 3, 30);
    CHECK("chain else", Chain(0, 0, 0), 40, 1, 2, 3, 40);

    CHECK("and skip", And(0, 1), 20, 1, 20);
    CHECK("and true", And(1, 1), 10, 1, 2, 10);
    CHECK("and false RHS", And(1, 0), 20, 1, 2, 20);
    CHECK("or skip", Or(1, 0), 10, 1, 10);
    CHECK("or true RHS", Or(0, 1), 10, 1, 2, 10);
    CHECK("or false", Or(0, 0), 20, 1, 2, 20);

    CHECK("nested skip middle", Nested(0, 1, 1), 10, 1, 3, 10);
    CHECK("nested skip last", Nested(1, 1, 0), 10, 1, 2, 10);
    CHECK("nested all", Nested(1, 0, 1), 10, 1, 2, 3, 10);
    CHECK("nested false", Nested(0, 1, 0), 20, 1, 3, 20);

    CHECK("logical chain first", LogicalChain(1, 1, 1, 1, 1), 10, 1, 2, 10);
    CHECK("logical chain OR skip", LogicalChain(0, 1, 1, 1, 1), 20, 1, 3, 20);
    CHECK("logical chain OR RHS", LogicalChain(0, 1, 0, 1, 1), 20, 1, 3, 4, 20);
    CHECK("logical chain third", LogicalChain(0, 1, 0, 0, 1), 30, 1, 3, 4, 5, 30);
    CHECK("logical chain else", LogicalChain(0, 1, 0, 0, 0), 40, 1, 3, 4, 5, 40);
    CHECK("logical chain all tests", LogicalChain(1, 0, 0, 0, 0), 40, 1, 2, 3, 4, 5, 40);
    puts("20 lazy if scenarios passed");
    return 0;
}
