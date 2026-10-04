export const AVATAR_ACCEPT = 'image/jpeg,image/png,image/gif'
export const AVATAR_MAX_SIZE = 2 * 1024 * 1024

/** This early feedback complements the server's image-content validation. */
export function avatarFileError(file: Pick<File, 'type' | 'size'>): 'type' | 'size' | undefined {
  if (!AVATAR_ACCEPT.split(',').includes(file.type) || file.size === 0) return 'type'

  if (file.size > AVATAR_MAX_SIZE) return 'size'
}
