import React, { useEffect, useRef, useState } from 'react';
import gsap from 'gsap';

export default function Preloader({ onComplete }) {
  const containerRef = useRef(null);
  const ringRef = useRef(null);
  const logoRef = useRef(null);
  const statusRef = useRef(null);
  const [statusText, setStatusText] = useState('INITIALIZING');

  useEffect(() => {
    const tl = gsap.timeline({
      onComplete: () => {
        gsap.to(containerRef.current, {
          opacity: 0,
          duration: 0.4,
          ease: 'power2.in',
          onComplete: () => onComplete?.()
        });
      }
    });

    // Phase 1: Ring appears and spins (already spinning via CSS)
    tl.fromTo(ringRef.current,
      { scale: 0, opacity: 0 },
      { scale: 1, opacity: 1, duration: 0.6, ease: 'elastic.out(1, 0.5)' }
    );

    // Phase 2: Logo text fades up
    tl.fromTo(logoRef.current,
      { y: 20, opacity: 0 },
      { y: 0, opacity: 1, duration: 0.5, ease: 'power3.out' },
      '-=0.2'
    );

    // Phase 3: Status text appears
    tl.fromTo(statusRef.current,
      { opacity: 0 },
      { opacity: 1, duration: 0.3 },
      '-=0.2'
    );

    // Phase 4: Status text changes
    tl.call(() => setStatusText('CONNECTING TO GATEWAY'), null, '+=0.5');
    tl.call(() => setStatusText('LOADING PROVIDERS'), null, '+=0.6');
    tl.call(() => setStatusText('READY'), null, '+=0.5');

    // Phase 5: Ring pulses then everything fades
    tl.to(ringRef.current, {
      scale: 1.3,
      opacity: 0,
      duration: 0.4,
      ease: 'power2.in'
    }, '+=0.3');

    return () => tl.kill();
  }, []);

  return (
    <div ref={containerRef} className="preloader">
      {/* Xbox-style spinning ring */}
      <div ref={ringRef} className="preloader-ring" />

      {/* Brand Logo */}
      <div ref={logoRef} className="preloader-logo">
        AI Router
      </div>

      {/* Status Text */}
      <div ref={statusRef} className="preloader-status">
        {statusText}
      </div>
    </div>
  );
}
