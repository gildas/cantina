# cantina
REST based File Storage Server

Upload stuff:

```
http --form POST http://cantina/api/v1/files?key=12345678 file@~/Downloads/picture.png
```

Download stuff:
```
http GET http://cantina/api/v1/files/picture.png
```

