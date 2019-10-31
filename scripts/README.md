Some useful scripts
-------------------

* **`mk-debian-image`** shows how to prepare a bootable raw image from an appropriate Docker image.

    It extends the specified Docker image by additional packages such as kernel image, initramfs tools, init system and grub loader.

    Resulting content of rootfs moving to the second partition of raw image, created using `qemu-img` util. 

    Then script configures the bootloader (`grub.cfg`, `mbr`) and a few additional parameters, such as hostname, root password, fstab, etc. 

    All these actions are performed in Docker containers.

    Dependencies: `docker-ce`, `qemu-utils`
