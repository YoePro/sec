// Foreign probes verify left-to-right, exact-once operand evaluation.
static int phase, left_count, right_count, bad;
static void left(void) { if (phase != 0) bad = 1; phase = 1; ++left_count; }
static void right(void) { if (phase != 1) bad = 1; phase = 0; ++right_count; }
float Left32(void) { left(); return 7.5f; }
float Right32(void) { right(); return 2.0f; }
double Left64(void) { left(); return 7.5; }
double Right64(void) { right(); return 2.0; }
int Check(void) { return bad || phase != 0 || left_count != 16 || right_count != 16; }
