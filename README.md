# Sec Programming Language

**"Safe by design, performant by nature, and readable by default."**

Sec is a modern, statically-typed systems programming language designed to bridge the gap between high-level domain safety and low-level hardware control. It combines the productivity of Go-like syntax with the rigorous safety and performance required for ERP systems, cloud services, and bare-metal embedded development.

---

## 🚀 Why Sec?

Sec is built for developers who need maximum control without the complexity or "PhD-level" knowledge required by other memory-safe languages.

*   **Safety without GC:** Built-in generational pointers provide memory safety without the non-deterministic pauses of a Garbage Collector.
*   **Domain-Driven Types:** First-class support for units (e.g., `decimal<m/s>`) and variable contracts (e.g., `range 0..100`) catches errors at the source.
*   **Zero-Boilerplate:** Clean, readable syntax that reduces cognitive load without sacrificing low-level expressiveness.
*   **Hardware-Ready:** Native support for memory-mapped registers and bit-level layout control.

---

## 💡 A Glimpse of the Syntax

Sec makes business logic and hardware integration intuitive:

```sec
type Speed decimal<m/s>

type Vehicle struct {
    _speed: Speed,
}

impl Vehicle {
    property TopSpeed: Speed {
        get { return _speed }
        
        try set value {
            if value < 0.0 { return Err(IOError.InvalidValue) }
            _speed = value
        }
    }
}

fn calculate_speed(dist: Meter, time: Second) Result[Speed, IOError] {
    if time <= 0.0 { return Err(IOError.InvalidValue) }
    return Ok(dist / time)
}