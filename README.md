# Pretty Console Output for Zap

This is an encoder for Uber's [zap][zap] logger that makes complex log output 
easily readable by humans. This is intended for development work where you 
don't have very many log messages and quickly understanding them is important.

This is not suitable for production, unless you never intend to 
automatically parse your own logs. Luckily zap makes it easy to switch to a 
different encoder in production 😁

## Current Status
This project is still under heavy development. The main branch should be in 
a working state, but it may have history edits. There is currently no 
documentation.

Released under the [MIT License](LICENSE.txt)

[zap]: https://github.com/uber-go/zap
