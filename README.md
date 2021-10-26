# cantina
REST based File Storage Server

## Downloading

Downloading content is very easy and does not require any credential, by default:

Using [httpie](https://httpie.io):
```console
http GET http://cantina/api/v1/files/picture.png > picture.png
```

If you prefer Good Ol' Curl:
```console
curl -sSLO GET http://cantina/api/v1/files/picture.png
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

If you prefer Good Ol' Curl:
```console
curl -H 'X-key:12345678' -F 'file=@myfile-0.0.1.min.js' https://cantina/upload
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


## Deleting

Deleting stuff using [httpie](https://httpie.io):

```console
http DELETE http://cantina/api/v1/files/picture.png X-Key:12345678
```

If you prefer Good Ol' Curl:
```console
curl -H 'X-key:12345678' -X DELETE https://cantina/upload/picture.png
```

Or with PowerShell:
```posh
Invoke-RestMethod https://cantina/api/v1/files/picture.png `
  -Method Delete `
  -Headers @{ 'X-Key' = '12345678' } `
```

**Note:** Thumbnail of the file is also deleted, though no error is returned if deletion failed.