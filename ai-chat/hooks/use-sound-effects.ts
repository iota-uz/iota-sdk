'use client';

import { useRef, useEffect, useCallback } from 'react';

interface SoundOptions {
  volume?: number;
  enabled?: boolean;
}

export function useSoundEffects(options: SoundOptions = {}) {
  const { volume = 0.5, enabled = true } = options;
  
  const submitSoundRef = useRef<HTMLAudioElement | null>(null);
  const operatorSoundRef = useRef<HTMLAudioElement | null>(null);
  
  useEffect(() => {
    if (typeof window !== 'undefined' && enabled) {
      submitSoundRef.current = new Audio('/sounds/submit.mp3');
      operatorSoundRef.current = new Audio('/sounds/operator.mp3');
      
      if (submitSoundRef.current) submitSoundRef.current.volume = volume;
      if (operatorSoundRef.current) operatorSoundRef.current.volume = volume;
      
      submitSoundRef.current?.load();
      operatorSoundRef.current?.load();
    }
    
    return () => {
      submitSoundRef.current = null;
      operatorSoundRef.current = null;
    };
  }, [enabled, volume]);
  
  const playSubmitSound = useCallback(() => {
    if (enabled && submitSoundRef.current) {
      submitSoundRef.current.currentTime = 0;
      submitSoundRef.current.play().catch(() => {});
    }
  }, [enabled]);
  
  const playOperatorSound = useCallback(() => {
    if (enabled && operatorSoundRef.current) {
      operatorSoundRef.current.currentTime = 0;
      operatorSoundRef.current.play().catch(() => {});
    }
  }, [enabled]);
  
  return {
    playSubmitSound,
    playOperatorSound
  };
}