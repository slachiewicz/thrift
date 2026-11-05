namespace cpp test.xception_field

exception Boom {
  1: string why
}

struct XceptionField {
  1: required Boom req_boom;
  2: Boom default_boom;
  3: optional Boom opt_boom;
  4: required i32 req_int;
}
