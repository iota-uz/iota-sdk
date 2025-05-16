'use client';

import { useRef, useEffect, useCallback } from 'react';

export interface SoundOptions {
  volume?: number;
  enabled?: boolean;
  submitSoundPath?: string;
  operatorSoundPath?: string;
}

export function useSoundEffects(options: SoundOptions = {}) {
  const { 
    volume = 0.5, 
    enabled = true,
    submitSoundPath = '/sounds/submit.mp3',
    operatorSoundPath = '/sounds/operator.mp3'
  } = options;
  
  const submitSoundRef = useRef<HTMLAudioElement | null>(null);
  const operatorSoundRef = useRef<HTMLAudioElement | null>(null);
  
  useEffect(() => {
    if (typeof window !== 'undefined' && enabled) {
      submitSoundRef.current = new Audio(submitSoundPath);
      operatorSoundRef.current = new Audio(operatorSoundPath);
      
      if (submitSoundRef.current) submitSoundRef.current.volume = volume;
      if (operatorSoundRef.current) operatorSoundRef.current.volume = volume;
      
      submitSoundRef.current?.load();
      operatorSoundRef.current?.load();
    }
    
    return () => {
      submitSoundRef.current = null;
      operatorSoundRef.current = null;
    };
  }, [enabled, volume, submitSoundPath, operatorSoundPath]);
  
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