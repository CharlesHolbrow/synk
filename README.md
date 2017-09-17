# Synk

Go library for syncing golang structures with the javascript clients.

## Confirmation Proposal

One proposal was to have an option for sending replies back to the simulation
loop. Handling `confirm/fail` messages from redis could help our program recover
from situations where the write failed. For example:

- We tried to create a new object, but an object with the same ID already exists
- We tried to modify an object, but the connection to redis was broken.

Below is my proposed addition to the existing system for handling `confirm/fail`
reply messages, and an explanation of why it would be non-trivial in the form
of an example failure case.

### A Simple Successful Example

```
{a:0, aDiff: nil}
    set a:1
{a:0, aDiff: 1}
    synk
{a:0, aDiff: 1}
    receive confirmation {aDiff: 1}
```

When we receive confirmation message, for each value in the confirmation
message:

- `if confMsg.aDiff == object.aDiff then object.aDiff = nil`
- `object.a = confMsg.aDiff`

If we do not receive confirmation, or we receive cancellation, we can do
nothing, and the new target value will still be in `aDiff`

### Failure Mode

In the above example, what happens if we receive a new set value before getting
the confirmation?

```
{a:0, aDiff: nil}
    set a:1
{a:0, aDiff: 1}
    synk
{a:0, aDiff: 1}
    set a:0
```

When we receive another `set` before receiving confirmation. Note that the set
returns `a` to its previous value.

- `if newA != object.a then object.aDiff == newA`
- `else object.aDiff == nil`

```
{a:0, aDiff: nil}
    receive confirmation {aDiff: 1}
{a: 1, aDiff: nil}
```

Now if we get failure message back from `set a:0` we are in a failure mode: The
target value for `a` is not stored in `aDiff`, because we didn't think we would
need it.

### Resolution

Because the vast majority of the time our writes to redis should be successful
I am opting to not implement confirmation messages. The simulation loop will
assume all writes are successful. If an error is encountered the simulation
should be notified, so it can stop, re-get the current state from redis, and
pick up where it left off.

Note that while there are workarounds to the failure mode described here, it
appears that these workarounds add considerable complexity or come with
different failure modes of their own.

In critical cases where a write must not fail, we should probably just wait
for the write to redis to complete.

There is still an outstanding issue. Imagine the following sequence:

1. We send a new Object request to redis for object with key `n:1`
1. We send a modification object request to redis for the `n:1` object
1. Our initial new object request fails due to existing object with same key
1. Our modification request succeeds, but modifies the wrong object.
1. Our simulation receives the error response for the first message

In this example, by the time the simulation was notified of the error, data in
redis has already been corrupted.

To avoid this, ensure that object keys have sufficient entropy OR wait for
redis to confirm object creation before modifying the object.
