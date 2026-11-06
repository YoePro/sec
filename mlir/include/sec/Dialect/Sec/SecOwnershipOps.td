#ifndef SEC_DIALECT_SEC_OWNERSHIPOPS_TD
#define SEC_DIALECT_SEC_OWNERSHIPOPS_TD

// Dialect v13 sections 4-13: explicit ownership transfer and destruction. Ownership
// stays SSA and Place identity plus these operations; no token type, destructor ABI
// or storage layout is selected. Path-sensitive state belongs to
// --sec-verify-ownership, not to these local contracts.
class Sec_OwnOp<string mnemonic, list<Trait> traits = []> : Sec_Op<mnemonic, traits> {
  let hasVerifier = 1;
}
def Sec_OwnCopyOp : Sec_OwnOp<"own.copy", [Pure]> {
  let summary = "Copy a copy-trivial value; the source stays valid";
  let arguments = (ins AnyType:$source);
  let results = (outs AnyType:$result);
}
def Sec_OwnSemanticCopyOp : Sec_OwnOp<"own.semantic_copy"> {
  let summary = "Copy through a resolved infallible semantic-copy plan";
  let arguments = (ins AnyType:$source, StrAttr:$semantic_copy_plan);
  let results = (outs AnyType:$result);
}
def Sec_OwnMoveOp : Sec_OwnOp<"own.move"> {
  let summary = "Consume ownership on this path; the result is its continuation";
  let arguments = (ins AnyType:$source);
  let results = (outs AnyType:$result);
}
def Sec_OwnCopyFromPlaceOp : Sec_OwnOp<"own.copy_from_place"> {
  let summary = "Copy out of a Place, which stays initialized";
  let arguments = (ins AnyType:$place, StrAttr:$copy_mode, OptionalAttr<StrAttr>:$semantic_copy_plan);
  let results = (outs AnyType:$result);
}
def Sec_OwnMoveFromPlaceOp : Sec_OwnOp<"own.move_from_place"> {
  let summary = "Move out of an owned initialized Place, which becomes uninitialized";
  let arguments = (ins AnyType:$place);
  let results = (outs AnyType:$result);
}
def Sec_OwnInitializePlaceOp : Sec_OwnOp<"own.initialize_place"> {
  let summary = "Make an uninitialized writable Place the owner of a value";
  let arguments = (ins AnyType:$place, AnyType:$value);
}
// `if_owned` acts only on runtime paths that still own the value, for a Place moved
// on some paths; the representation of that state is lowering's choice
// (destruction.md sections 11.2, 13.3, 35.5).
def Sec_OwnReplacePlaceOp : Sec_OwnOp<"own.replace_place"> {
  let summary = "Replace an initialized Place's value with a fully prepared one";
  let arguments = (ins AnyType:$place, AnyType:$value, StrAttr:$destruction_plan,
    I32Attr:$old_cleanup_action_id, I32Attr:$new_cleanup_action_id, UnitAttr:$if_owned);
}
def Sec_OwnDiscardOp : Sec_OwnOp<"own.discard"> {
  let summary = "Consume an owned value or end a held reference without destroying its referent";
  let arguments = (ins AnyType:$value, StrAttr:$discard_origin, OptionalAttr<StrAttr>:$destruction_plan, UnitAttr:$if_owned);
}
def Sec_OwnDestroyValueOp : Sec_OwnOp<"own.destroy_value"> {
  let summary = "Destroy an owned SSA value through its type-level plan";
  let arguments = (ins AnyType:$value, StrAttr:$destruction_plan, StrAttr:$cause);
}
def Sec_OwnDestroyPlaceOp : Sec_OwnOp<"own.destroy_place"> {
  let summary = "Destroy an owned initialized Place, except subobjects already moved out";
  let arguments = (ins AnyType:$place, StrAttr:$destruction_plan, StrAttr:$cause,
    OptionalAttr<StrArrayAttr>:$skipped, UnitAttr:$if_owned);
}

// Dialect v13 sections 14-22: cleanup registration and execution. Identities are
// compile-time attributes; no cleanup stack, registry or landing pad is selected.
class Sec_CleanupOp<string mnemonic, list<Trait> traits = []> : Sec_Op<mnemonic, traits> {
  let hasVerifier = 1;
}
def Sec_CleanupTrackOwnedOp : Sec_CleanupOp<"cleanup.track_owned"> {
  let summary = "Register an automatic destruction responsibility for a value or Place";
  // unless_active (MD-125): register only on paths where none of these
  // actions is live; elsewhere that inherited registration keeps its position.
  let arguments = (ins AnyType:$tracked, I32Attr:$cleanup_action_id, I32Attr:$cleanup_scope_id,
    I32Attr:$cleanup_registration_ordinal, StrAttr:$destruction_plan,
    OptionalAttr<DenseI32ArrayAttr>:$unless_active);
}
def Sec_CleanupCancelOp : Sec_CleanupOp<"cleanup.cancel"> {
  let summary = "Cancel one active automatic cleanup responsibility";
  let arguments = (ins I32Attr:$cleanup_action_id, StrAttr:$reason, UnitAttr:$if_active);
}
def Sec_CleanupDeferRegisterOp : Sec_CleanupOp<"cleanup.defer_register"> {
  let summary = "Register one defer instance each time this operation executes";
  let arguments = (ins Variadic<AnyType>:$captures, I32Attr:$cleanup_action_id,
    I32Attr:$cleanup_scope_id, I32Attr:$cleanup_registration_ordinal, I32Attr:$defer_id);
  let regions = (region AnyRegion:$body);
}
def Sec_CleanupDeferYieldOp : Sec_CleanupOp<"cleanup.defer_yield", [Terminator]> {
  let summary = "Return from a defer body to cleanup execution";
}
def Sec_CleanupRunScopeOp : Sec_CleanupOp<"cleanup.run_scope"> {
  let summary = "Run eligible automatic cleanup of exited lexical scopes";
  let arguments = (ins DenseI32ArrayAttr:$scope_ids, StrAttr:$exit_kind);
}
def Sec_CleanupRunFunctionOp : Sec_CleanupOp<"cleanup.run_function"> {
  let summary = "Run remaining cleanup entries at a language-controlled function exit";
  let arguments = (ins StrAttr:$exit_kind);
}

#endif
