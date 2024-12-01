# cantina

Very simple REST based File Storage Server

## Installation

If you already run Go,

```bash
go install github.com/gildas/cantina@latest
```

Using Docker:

```bash
docker run -d -p 80:8080 -v /path/to/storage:/var/storage gildas/cantina
```

## Operation

You can access cantina via simple REST commands in order to upload, delete, download files.

Typically, downloads are anonymous and uploads/deletes are secured by keys.

## Downloading

Downloading content is very easy and does not require any credential (by default):

Using [httpie](https://httpie.io):

```bash
http --download http://cantina/api/v1/files/picture.png
```

Curl:

```bash
curl -sSLO http://cantina/api/v1/files/picture.png
```

Or with PowerShell:

```posh
Invoke-RestMethod https://cantina/api/v1/files/picture.png -OutFile picture.png
```

If the file is protected by a password, you will need to provide it, either in the query (**not really** secure!) or in the headers (`Authorization` header):

```bash
http --download http://cantina/api/v1/files/picture.png?password=secret
http --auth-type=bearer --auth=secret --download http://cantina/api/v1/files/picture.png
```

```bash
curl -sSLO http://cantina/api/v1/files/picture.png?password=secret
curl -sSLO http://cantina/api/v1/files/picture.png -H 'Authorization: bearer secret'
```

```posh
Invoke-RestMethod https://cantina/api/v1/files/picture.png?password=secret `
  -OutFile picture.png
Invoke-RestMethod https://cantina/api/v1/files/picture.png `
  -OutFile picture.png `
  -Headers @{ 'Authorization='bearer secret' }
```

If the file has versions, you can specify the version you want to download:

```bash
http --download http://cantina/api/v1/files/picture.png?version=1
```

```bash
curl -sSLO http://cantina/api/v1/files/picture.png?version=1
```

```posh
Invoke-RestMethod https://cantina/api/v1/files/picture.png?version=1 `
  -OutFile picture.png
```

You can ask for the `latest` version as well.

## Uploading

Upload stuff using [httpie](https://httpie.io):

```bash
http --auth-type=bearer --auth=12345678 --form POST http://cantina/api/v1/files file@~/Downloads/picture.png
http --form POST http://cantina/api/v1/files X-Key:12345678 file@~/Downloads/picture.png
```

You can also pass your key in the query, which is less secure:

```bash
http --form POST http://cantina/api/v1/files?key=12345678 file@~/Downloads/picture.png
```

Curl:

```bash
curl -H 'Authorization: bearer 12345678' -F 'file=@myfile-0.0.1.min.js' https://cantina/upload
curl -H 'X-key:12345678' -F 'file=@myfile-0.0.1.min.js' https://cantina/upload
curl -F 'file=@myfile-0.0.1.min.js' https://cantina/upload?key=12345678
```

Or with PowerShell:

```posh
Invoke-RestMethod https://cantina/api/v1/files `
  -Method Post `
  -Headers @{ 'Authorization' = 'bearer 12345678' } `
  -Form @{ file = Get-Item ./picture.png }
Invoke-RestMethod https://cantina/api/v1/files `
  -Method Post `
  -Headers @{ 'X-Key' = '12345678' } `
  -Form @{ file = Get-Item ./picture.png }
```

You can also set a time for the file to be automatically deleted (purged):

- The form value `purgeAfter` will delete the file after the given delay.  
  Synonyms: purgeIn, deleteAfter, deleteIn.
- The form value `purgeOn` will delete the file on the give datetime.  
  Synonyms: purgeAt, deleteAt

For example:

```bash
http --form POST http://cantina/api/v1/files \
  X-Key:12345678 \
  file@~/Downloads/picture.png \
  purgeIn=1h
```
  
```bash
curl -H 'X-key:12345678' \
  -F 'file=@myfile-0.0.1.min.js' \
  -F 'purgeIn=1h' \
  https://cantina/upload
```

```posh
Invoke-RestMethod https://cantina/api/v1/files `
  -Method Post `
  -Headers @{ 'X-Key' = '12345678' } `
  -Form @{ file = Get-Item ./picture.png; purgeIn = '1h' }
```

You can also set a key or a password to protect the file:

- The form value `password` will protect the file with the given password.  

For example:

```bash
http --form POST http://cantina/api/v1/files \
  X-Key:12345678 \
  file@~/Downloads/picture.png \
  password=secret
```
  
```bash
curl -H 'X-key:12345678' \
  -F 'password=secret' \
  https://cantina/upload
```

```posh
Invoke-RestMethod https://cantina/api/v1/files `
  -Method Post `
  -Headers @{ 'X-Key' = '12345678' } `
  -Form @{ file = Get-Item ./picture.png; password='secret' }
```

You can configure the purge delay and the password after the file has been uploaded using the `PATCH` method:

```bash
http --form PATCH http://cantina/api/v1/files/picture.png \
  X-Key:12345678 \
  purgeAfter=1h
  password=secret
```

```bash
curl -H 'X-key:12345678' \
  -F 'purgeIn=1h' \
  -F 'password=secret' \
  https://cantina/upload/picture.png
```

```posh
Invoke-RestMethod https://cantina/api/v1/files/picture.png `
  -Method Patch `
  -Headers @{ 'X-Key' = '12345678' } `
  -Form @{ purgeAt = '2024-12-31T23:59:59'; password='secret' }
```

## Deleting

Deleting stuff using [httpie](https://httpie.io):

```bash
http DELETE http://cantina/api/v1/files/picture.png X-Key:12345678
```

Curl:

```bash
curl -H 'X-key:12345678' -X DELETE https://cantina/upload/picture.png
```

Or with PowerShell:

```posh
Invoke-RestMethod https://cantina/api/v1/files/picture.png `
  -Method Delete `
  -Headers @{ 'X-Key' = '12345678' } `
```

**Note:** If the file had a thumbnail (images, etc), it is also deleted.
