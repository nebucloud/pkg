package logger

import (
	"context"
	"fmt"
	"testing"
)

func TestProduction(t *testing.T) {
	InitFlags(nil)
	klogger.config.v = 1 // enable DEBUG level
	Singleton()

	arg := fmt.Errorf("hello")
	arg2 := fmt.Errorf("world")
	Error(arg)
	Errorf("%s", arg)
	Errorln(arg, arg2)
	ErrorDepth(1, arg)
	Warningf("%s", arg)
	Warning(arg)
	Warningln(arg, arg2)
	WarningDepth(1, arg)
	Infof("%s", arg)
	Infoln(arg, arg2)
	InfoDepth(1, arg)
	Info(arg)
	V(1).Info(arg)
	V(1).Infoln(arg)
	V(1).Infof("%s", arg)
	V(2).Info(arg)
	V(2).Infoln(arg)
	V(2).Infof("%s", arg)
}

func TestWith(t *testing.T) {
	Singleton()
	type S struct {
		A int
		B string
	}
	type ID int64
	type W struct {
		C S
		D ID
	}
	type Q struct {
		D ID
	}
	c := "hello"
	s := S{
		A: 10,
		B: "abc",
	}
	w := W{
		C: s,
		D: ID(1),
	}
	q := Q{
		D: ID(1),
	}
	fields := map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	}
	l1 := WithFields(fields)
	l1.Info(context.Background(), "hello")
	WithFields(map[string]interface{}{
		"A": 10,
		"B": "abc",
	}).Info(context.Background(), c)
	With("A", 10, "B", "abc").Info(context.Background(), c)
	WithAll(s).Info(context.Background(), c)
	With("C", s, "D", w.D).Info(context.Background(), c)
	WithAll(w).Info(context.Background(), c)
	With("A", s.A, "B", s.B, "D", q.D).Info(context.Background(), c)
	WithAll(s, q).Info(context.Background(), c)
	With("A", 1, "B", 2).Info(context.Background(), c)
	WithAll(struct {
		A int
		B int
	}{1, 2}).Info(context.Background(), c)
	type Y map[string]string
	y := Y{"a": "b", "c": "d"}
	With("Y", y, "c", c, "map", map[struct{ A string }]int{{A: "a"}: 1, {A: "b"}: 2}).Info(context.Background(), c)
	WithAll(y, c, map[struct{ A string }]int{{A: "a"}: 1, {A: "b"}: 2}).Info(context.Background(), c)
}

func TestNoOps(t *testing.T) {
	arg := fmt.Errorf("hello")
	arg2 := fmt.Errorf("world")
	Error(arg)
	Errorf("%s", arg)
	Errorln(arg, arg2)
	ErrorDepth(1, arg)
	Warningf("%s", arg)
	Warning(arg)
	Warningln(arg, arg2)
	WarningDepth(1, arg)
	Infof("%s", arg)
	Infoln(arg, arg2)
	InfoDepth(1, arg)
	Info(arg)
	V(1).Info(arg)
	V(1).Infoln(arg)
	V(1).Infof("%s", arg)
	V(2).Info(arg)
	V(2).Infoln(arg)
	V(2).Infof("%s", arg)
}

func TestUpdateLevel(t *testing.T) {
	Singleton()
	Infof("should-always-print")
	V(1).Infof("should-not-print")
	SetLevel(2)
	SetLevel(5)
	SetLevel(-1)
	V(1).Infof("should-print")
}

func BenchmarkWith(b *testing.B) {
	Singleton()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		With(struct {
			ID   string
			Name string
		}{"0001", "hello"}).Info("world")
	}
}

func BenchmarkWithFields(b *testing.B) {
	Singleton()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithFields(map[string]interface{}{
			"ID":   "0001",
			"Name": "hello",
		}).Info(context.Background(), "world")
	}
}

func BenchmarkWithAll(b *testing.B) {
	Singleton()
	type s struct {
		ID   string
		Name string
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WithAll(s{"0001", "hello"}).Info("world")
	}
}

func BenchmarkContextWith(b *testing.B) {
	Singleton()
	type s struct {
		ID   string
		Name string
	}
	b.ResetTimer()
	newLogger := With(struct {
		ID   string
		Name string
	}{"0001", "hello"})
	for i := 0; i < b.N; i++ {
		newLogger.Info("world")
	}
}
