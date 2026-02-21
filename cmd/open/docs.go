/*
`open` launches a file or directory using app associations.

How it works:
- Files are opened by extension-to-app mapping.
- Directories open in File Manager.
- You can force a specific app with `--app=<id>`.

Example usage:
```sh
open ./notes.txt
open ./photos/cat.png
open ./projects --app=dev.avyos.filemanager
```
*/
package main
