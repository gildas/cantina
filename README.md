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
