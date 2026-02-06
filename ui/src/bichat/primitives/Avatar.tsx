/**
 * Avatar Primitive
 * Compound component for displaying user avatars with image and fallback
 */

import {
  createContext,
  useContext,
  useState,
  forwardRef,
  type HTMLAttributes,
  type ImgHTMLAttributes,
} from 'react'
import { Slot, type AsChildProps } from './Slot'

/* -------------------------------------------------------------------------------------------------
 * AvatarContext
 * -----------------------------------------------------------------------------------------------*/

type ImageLoadingStatus = 'idle' | 'loading' | 'loaded' | 'error'

interface AvatarContextValue {
  imageLoadingStatus: ImageLoadingStatus
  setImageLoadingStatus: (status: ImageLoadingStatus) => void
}

const AvatarContext = createContext<AvatarContextValue | undefined>(undefined)

function useAvatarContext() {
  const context = useContext(AvatarContext)
  if (!context) {
    throw new Error('Avatar components must be used within Avatar.Root')
  }
  return context
}

/* -------------------------------------------------------------------------------------------------
 * Avatar.Root
 * -----------------------------------------------------------------------------------------------*/

type AvatarRootProps = AsChildProps<HTMLAttributes<HTMLSpanElement>>

const AvatarRoot = forwardRef<HTMLSpanElement, AvatarRootProps>((props, ref) => {
  const { asChild, children, ...domProps } = props
  const Comp = asChild ? Slot : 'span'
  const [imageLoadingStatus, setImageLoadingStatus] = useState<ImageLoadingStatus>('idle')

  return (
    <AvatarContext.Provider value={{ imageLoadingStatus, setImageLoadingStatus }}>
      <Comp ref={ref} {...domProps}>
        {children}
      </Comp>
    </AvatarContext.Provider>
  )
})

AvatarRoot.displayName = 'Avatar.Root'

/* -------------------------------------------------------------------------------------------------
 * Avatar.Image
 * -----------------------------------------------------------------------------------------------*/

type AvatarImageProps = AsChildProps<ImgHTMLAttributes<HTMLImageElement>> & {
  /** Called when loading status changes */
  onLoadingStatusChange?: (status: ImageLoadingStatus) => void
}

const AvatarImage = forwardRef<HTMLImageElement, AvatarImageProps>((props, ref) => {
  const { asChild, src, alt, onLoadingStatusChange, onLoad, onError, ...domProps } = props
  const { setImageLoadingStatus } = useAvatarContext()
  const Comp = asChild ? Slot : 'img'

  const handleLoad = (e: React.SyntheticEvent<HTMLImageElement>) => {
    setImageLoadingStatus('loaded')
    onLoadingStatusChange?.('loaded')
    onLoad?.(e)
  }

  const handleError = (e: React.SyntheticEvent<HTMLImageElement>) => {
    setImageLoadingStatus('error')
    onLoadingStatusChange?.('error')
    onError?.(e)
  }

  // Start loading when src is provided
  if (src) {
    return (
      <Comp
        ref={ref}
        src={src}
        alt={alt || ''}
        onLoad={handleLoad}
        onError={handleError}
        {...domProps}
      />
    )
  }

  return null
})

AvatarImage.displayName = 'Avatar.Image'

/* -------------------------------------------------------------------------------------------------
 * Avatar.Fallback
 * -----------------------------------------------------------------------------------------------*/

type AvatarFallbackProps = AsChildProps<HTMLAttributes<HTMLSpanElement>> & {
  /** Delay before showing fallback (in ms) */
  delayMs?: number
}

const AvatarFallback = forwardRef<HTMLSpanElement, AvatarFallbackProps>((props, ref) => {
  const { asChild, delayMs = 0, children, ...domProps } = props
  const { imageLoadingStatus } = useAvatarContext()
  const Comp = asChild ? Slot : 'span'
  const [canRender, setCanRender] = useState(delayMs === 0)

  // Handle delay
  if (delayMs > 0 && !canRender) {
    setTimeout(() => setCanRender(true), delayMs)
  }

  // Only show fallback if image hasn't loaded
  if (imageLoadingStatus === 'loaded') {
    return null
  }

  if (!canRender) {
    return null
  }

  return (
    <Comp ref={ref} {...domProps}>
      {children}
    </Comp>
  )
})

AvatarFallback.displayName = 'Avatar.Fallback'

/* -------------------------------------------------------------------------------------------------
 * Exports
 * -----------------------------------------------------------------------------------------------*/

export const Avatar = {
  Root: AvatarRoot,
  Image: AvatarImage,
  Fallback: AvatarFallback,
}

export { useAvatarContext }
export type { AvatarRootProps, AvatarImageProps, AvatarFallbackProps, ImageLoadingStatus }
