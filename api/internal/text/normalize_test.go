package text

import "testing"

func TestNormalizeQuestion(t *testing.T) {
    cases := []struct{
        in  string
        out string
    }{
        {" Hello, WORLD!!!  ", "hello world"},
        {"What‘s up?  ", "what s up"},
        {"A  B\tC\nD", "a b c d"},
        {"Numbers: 123, 456.", "numbers 123 456"},
    }
    for i, c := range cases {
        got := NormalizeQuestion(c.in)
        if got != c.out {
            t.Fatalf("case %d: got %q want %q", i, got, c.out)
        }
    }
}

