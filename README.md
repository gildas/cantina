# cantina
REST based File Storage Server

## Sending to the server

Example with [httpie](https://httpie.org):  
```console
http --verbose --form POST http://localhost:3000/api/v1/files X-Key:1234 Origin:https://www.acme.com  file@~/Dropbox/Photos/avatars/avatar-tux-R2D2.png
```