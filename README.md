# IPFS Jackal Uploader

Launches a folder or file to Jackal & IPFS!

## Install
```shell
git clone https://github.com/TheMarstonConnell/ipfs-uploader.git
cd ipfs-uploader
go install .
```

## Usage
To start, copy `.env.example` to `.env` and fill it out with your configuration and seed phrase. Then run these commands beside your `.env` file.

```shell
ipfsUploader launch {folder-path}
```

Or for a single file:
```shell
ipfsUploader blast {file-path}
```

If the binary crashes at any time, run it again, and it won't re-upload already uploaded files, but it will still track them for the IPFS folder creation. 