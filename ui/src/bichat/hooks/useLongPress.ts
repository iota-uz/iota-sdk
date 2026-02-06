import { useRef, useState, useCallback, useEffect } from 'react';

interface LongPressOptions {
  delay?: number; // Default: 500ms
  onLongPress: (e: React.TouchEvent | React.MouseEvent) => void;
  onPressStart?: () => void;
  onPressCancel?: () => void;
  moveThreshold?: number; // Default: 10px - cancel if moved more
  hapticFeedback?: boolean; // Default: true
}

interface LongPressEventHandlers {
  onTouchStart: (e: React.TouchEvent) => void;
  onTouchEnd: (e: React.TouchEvent) => void;
  onTouchMove: (e: React.TouchEvent) => void;
  onMouseDown?: (e: React.MouseEvent) => void; // For desktop testing
  onMouseUp?: (e: React.MouseEvent) => void;
  onMouseLeave?: (e: React.MouseEvent) => void;
}

interface LongPressResult {
  handlers: LongPressEventHandlers;
  isPressed: boolean;
}

export function useLongPress(options: LongPressOptions): LongPressResult {
  const {
    delay = 500,
    onLongPress,
    onPressStart,
    onPressCancel,
    moveThreshold = 10,
    hapticFeedback = true,
  } = options;

  const [isPressed, setIsPressed] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const startPosRef = useRef<{ x: number; y: number } | null>(null);
  const eventRef = useRef<React.TouchEvent | React.MouseEvent | null>(null);

  const clearTimer = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const handlePressStart = useCallback(
    (e: React.TouchEvent | React.MouseEvent) => {
      // Note: Do NOT call preventDefault() on touchstart - it breaks long-press on iPadOS Safari
      // Text selection is prevented via CSS user-select: none on .touch-tap class instead

      setIsPressed(true);
      eventRef.current = e;

      // Store starting position
      const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
      const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY;
      startPosRef.current = { x: clientX, y: clientY };

      onPressStart?.();

      clearTimer();
      timerRef.current = setTimeout(() => {
        if (hapticFeedback && navigator.vibrate) {
          navigator.vibrate(10);
        }
        onLongPress(eventRef.current!);
      }, delay);
    },
    [delay, onLongPress, onPressStart, hapticFeedback, clearTimer]
  );

  const handlePressEnd = useCallback(() => {
    clearTimer();
    setIsPressed(false);
    startPosRef.current = null;
    eventRef.current = null;
  }, [clearTimer]);

  const handlePressCancel = useCallback(() => {
    clearTimer();
    setIsPressed(false);
    startPosRef.current = null;
    eventRef.current = null;
    onPressCancel?.();
  }, [clearTimer, onPressCancel]);

  const handleMove = useCallback(
    (e: React.TouchEvent | React.MouseEvent) => {
      if (!startPosRef.current || !isPressed) return;

      const clientX = 'touches' in e ? e.touches[0].clientX : e.clientX;
      const clientY = 'touches' in e ? e.touches[0].clientY : e.clientY;

      const deltaX = Math.abs(clientX - startPosRef.current.x);
      const deltaY = Math.abs(clientY - startPosRef.current.y);

      if (deltaX > moveThreshold || deltaY > moveThreshold) {
        handlePressCancel();
      }
    },
    [isPressed, moveThreshold, handlePressCancel]
  );

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      clearTimer();
    };
  }, [clearTimer]);

  return {
    handlers: {
      onTouchStart: handlePressStart,
      onTouchEnd: handlePressEnd,
      onTouchMove: handleMove,
      onMouseDown: handlePressStart,
      onMouseUp: handlePressEnd,
      onMouseLeave: handlePressCancel,
    },
    isPressed,
  };
}
