namespace cpp test.service_helpers

struct Payload {
  1: required string name;
}

exception Oops {
  1: string why;
}

service HelperService {
  Payload echo(1: Payload p, 2: list<string> names, 3: string text) throws (1: Oops oops);
  void ping(1: i32 n);
  oneway void fire(1: string text);
}
