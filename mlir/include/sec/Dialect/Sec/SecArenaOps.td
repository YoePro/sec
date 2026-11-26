#ifndef SEC_DIALECT_SEC_ARENAOPS_TD
#define SEC_DIALECT_SEC_ARENAOPS_TD

// SEC-MLIR-P19 sections 105-123: schema-15 Arena owners and allocation
// contexts. The operations carry the resolved Semantic IR facts: the Arena
// domain and storage classification reuse the P18 backing facts; state
// versions order mutations and epochs validate references (section 121);
// plan names reference resolved provider/capacity/layout/default/dependency
// decisions. No allocator, cursor, layout or ABI is selected here. Every
// operation declares its memory effects on the Sec Arena resource (§ 123):
// creation allocates, New/Alloc read, write and allocate, Reset reads and
// writes, Release/destroy free; context selection reads. Allocation is
// therefore never CSE'd, hoisted or moved across Reset/Release, and Reset is
// not an ordinary Free.

defvar Sec_ArenaFacts = (ins
    OptionalAttr<I32Attr>:$state_before,
    I32Attr:$state_after,
    OptionalAttr<I32Attr>:$epoch_before,
    OptionalAttr<I32Attr>:$epoch_after,
    StrAttr:$backing_kind,
    StrAttr:$growth_kind,
    OptionalAttr<StrAttr>:$provider_plan,
    OptionalAttr<StrAttr>:$capacity_plan,
    OptionalAttr<StrAttr>:$layout_plan,
    OptionalAttr<StrAttr>:$default_plan,
    OptionalAttr<StrAttr>:$dependency_proof,
    OptionalAttr<I32Attr>:$retained_borrow,
    OptionalAttr<I32Attr>:$parent_storage,
    OptionalAttr<I32Attr>:$parent_domain,
    OptionalAttr<I32Attr>:$parent_epoch,
    UnitAttr:$failure_atomic);

// Section 110: a fixed Arena over a borrowed mutable byte slice.
def Sec_ArenaCreateBorrowedOp : Sec_Op<"arena.create_borrowed", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Create an Arena over borrowed mutable byte storage";
  let arguments = !con((ins AnyType:$backing), Sec_ArenaFacts, Sec_BackingFacts);
  let results = (outs AnyType:$arena);
  let hasVerifier = 1;
}

// MD-136 (P19 § 166): the empty default Arena. A fresh owned domain with no
// backing and zero capacity; infallible and allocation-free. It still
// declares an Allocate effect on the Arena resource so two defaults, which
// own distinct domains, are never merged.
def Sec_ArenaCreateDefaultOp : Sec_Op<"arena.create_default", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Create the empty default Arena (no backing, zero capacity)";
  let arguments = !con((ins), Sec_ArenaFacts, Sec_BackingFacts);
  let results = (outs AnyType:$arena);
  let hasVerifier = 1;
}

// Sections 111-112: an owned fixed or growable Arena from a byte capacity
// and the selected allocation context; checked result shape.
class Sec_ArenaCreateOwnedOp<string mnemonic, string text> : Sec_Op<mnemonic, [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = text;
  let arguments = !con((ins AnyType:$capacity, AnyType:$context), Sec_ArenaFacts, Sec_BackingFacts);
  let results = (outs AnyType:$arena, I1:$failed, AnyType:$error);
  let hasVerifier = 1;
}
def Sec_ArenaCreateOwnedFixedOp : Sec_ArenaCreateOwnedOp<"arena.create_owned_fixed",
    "Create an owned fixed-capacity Arena (checked)">;
def Sec_ArenaCreateGrowableOp : Sec_ArenaCreateOwnedOp<"arena.create_growable",
    "Create an owned growable non-relocating Arena (checked)">;

// Sections 113-114: atomic typed allocation through the Arena Place.
def Sec_ArenaNewOp : Sec_Op<"arena.new", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Allocate one default-initialized T in an Arena (checked)";
  let arguments = !con((ins AnyType:$arena), Sec_ArenaFacts, Sec_BackingFacts);
  let results = (outs AnyType:$reference, I1:$failed, AnyType:$error);
  let hasVerifier = 1;
}
def Sec_ArenaAllocOp : Sec_Op<"arena.alloc", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Allocate count default-initialized T in an Arena (checked)";
  let arguments = !con((ins AnyType:$arena, AnyType:$count), Sec_ArenaFacts, Sec_BackingFacts);
  let results = (outs AnyType:$slice, I1:$failed, AnyType:$error);
  let hasVerifier = 1;
}

// Section 115: advance the epoch of the Arena domain; not an ordinary Free.
// MD-140 (v15 section 11): the epoch-advance form, logical width and the
// registered exhaustion reason; the epoch stays a logical attribute here.
def Sec_ArenaResetOp : Sec_Op<"arena.reset", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Reset an Arena, invalidating every reference into it";
  let arguments = !con((ins AnyType:$arena, StrAttr:$epoch_advance, I32Attr:$epoch_width,
                            StrAttr:$exhaustion_reason, OptionalAttr<StrAttr>:$epoch_proof),
                       Sec_ArenaFacts, Sec_BackingFacts);
  let hasVerifier = 1;
}

// Sections 116-117: terminal consumption of the owner; destroy is the
// implicit-destruction form with its P17 destruction plan and cause.
def Sec_ArenaReleaseOp : Sec_Op<"arena.release", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Release an Arena owner and end its domain";
  let arguments = !con((ins AnyType:$arena), Sec_ArenaFacts, Sec_BackingFacts);
  let hasVerifier = 1;
}
def Sec_ArenaDestroyOp : Sec_Op<"arena.destroy", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Implicitly destroy an Arena owner (terminal Release)";
  let arguments = !con((ins AnyType:$arena, StrAttr:$destruction_plan, StrAttr:$cause), Sec_ArenaFacts, Sec_BackingFacts);
  let hasVerifier = 1;
}

// Sections 118-120: compiler-internal allocation capabilities with their
// resolved origin plan; none discovers a lexical Arena.
def Sec_AllocContextFromArenaOp : Sec_Op<"alloc_context.from_arena", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Select an explicitly named Arena as allocation context";
  let arguments = (ins AnyType:$arena, StrAttr:$origin_plan, I32Attr:$arena_domain);
  let results = (outs AnyType:$context);
  let hasVerifier = 1;
}
def Sec_AllocContextTargetOp : Sec_Op<"alloc_context.target", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Produce the target-provided allocation context at an entry root";
  let arguments = (ins StrAttr:$origin_plan);
  let results = (outs AnyType:$context);
  let hasVerifier = 1;
}
def Sec_AllocContextCompilerLocalOp : Sec_Op<"alloc_context.compiler_local", [DeclareOpInterfaceMethods<MemoryEffectsOpInterface>]> {
  let summary = "Produce a proven compiler-managed local allocation context";
  let arguments = (ins StrAttr:$origin_plan);
  let results = (outs AnyType:$context);
  let hasVerifier = 1;
}

#endif
