# cantina

Very simple REST based File Storage Server

## Installation

If you already run Go,

```bash
go install github.com/gildas/cantina@latest
```

Using Docker:

```bash
docker run -d -p 80:8080 -v /path/to/storage:/usr/local/storage gildas/cantina
```

## Operation

You can access cantina via simple REST commands in order to upload, delete, download files.

Typically, downloads are anonymous and uploads/deletes are secured by keys.

## Downloading

Downloading content is very easy and does not require any credential (by default):

Using [httpie](https://httpie.io):

```console
http GET http://cantina/api/v1/files/picture.png > picture.png
```

Curl:

```console
curl -sSLO http://cantina/api/v1/files/picture.png
```

Or with PowerShell:

```posh
Invoke-RestMethod https://cantina/api/v1/files/picture.png -OutFile picture.png
```

## Uploading

Upload stuff using [httpie](https://httpie.io):

```console
http --form POST http://cantina/api/v1/files X-Key:12345678 file@~/Downloads/picture.png
```

You can also pass your key in the query, which is less secure:

```console
http --form POST http://cantina/api/v1/files?key=12345678 file@~/Downloads/picture.png
```

Curl:

```console
curl -H 'X-key:12345678' -F 'file=@myfile-0.0.1.min.js' https://cantina/upload
curl -F 'file=@myfile-0.0.1.min.js' https://cantina/upload?key=12345678
```

Or with PowerShell:

```posh
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

```console
http --form POST http://cantina/api/v1/files \
  X-Key:12345678 \
  file@~/Downloads/picture.png \
  purgeIn=1h
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

## Deleting

Deleting stuff using [httpie](https://httpie.io):

```console
http DELETE http://cantina/api/v1/files/picture.png X-Key:12345678
```

Curl:

```console
curl -H 'X-key:12345678' -X DELETE https://cantina/upload/picture.png
```

Or with PowerShell:

```posh
Invoke-RestMethod https://cantina/api/v1/files/picture.png `
  -Method Delete `
  -Headers @{ 'X-Key' = '12345678' } `
```

**Note:** If the file had a thumbnail (images, etc), it is also deleted.
