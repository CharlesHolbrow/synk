# Synk

Go library for syncing golang structures on the server with the JavaScript objects on the client.

**NOTE:** This library is experimental. It is not production ready.

Synk is made to facilitate the development of real-time stateful web applications optimized for performance and scalability. The goal is to scale browser based worlds like https://aether.media.mit.edu/ -- by scaling both the number of concurrent connections, and also the size of the map.

Synk is not a web server. It a library to make the painful parts of writing a real-time stateful web application easier. Here's now a fully implemented synk server works:

- When the server starts, it loads a subset of documents stored in a mongodb server, and converts these objects to golang structures that implement the `synk.Object` interface.
- These `synk.Objects` are then in memory on the server. When you mutate the object the diff between the new state and the old state is saved.
- On the server, call `func (ms *synk.MongoSynk) Modify(obj synk.Object) error` function, providing a `*sync.Object` as an argument.
- If the object was modified since it was created
  1. The new object is serialized and the associated mongodb document is updated
  1. The "diff" between the old and new object is sent to any connected browser clients via websocket connection.

Meanwhile, on the client side (in the web browser)

- A sync client makes a websocket connection to the synk server using the `synk-js` npm package.
- The client 'subscribes' to zero or more subscription keys.
- The server sends the current state of any objects within the provided subscription keys.
- The client constructs a JavaScript objects for each synk object. As the application developer, you create this object by extending the provided class.

This library is made to handle certain certain cases that other libraries are not:

- Assume that the client is always changing their subscription key set.
- Assume that synk objects may move. The object's subscription key may be updated. - Ensure that client cache stays up to date, and no objects are 'lost' as a result of the client changing.

You (the developer) are responsible for implementing the required go interfaces. And javascript constructors.
