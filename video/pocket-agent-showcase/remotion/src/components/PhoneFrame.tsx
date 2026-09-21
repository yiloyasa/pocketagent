import React from "react";
import {Img, interpolate, staticFile} from "remotion";
import {Video} from "@remotion/media";
import {COLORS, FONTS} from "../styles/colors";

type PhoneFrameProps = {
  frame: number;
  startFrame: number;
  durationInFrames: number;
  useLivePhoneCapture: boolean;
};

export const PhoneFrame: React.FC<PhoneFrameProps> = ({
  frame,
  startFrame,
  durationInFrames,
  useLivePhoneCapture,
}) => {
  const local = frame - startFrame;
  const intro = interpolate(local, [0, 38], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const pushMidpoint = Math.max(70, Math.min(220, durationInFrames - 30));
  const pushIn = interpolate(local, [42, pushMidpoint, durationInFrames], [0.9, 1.03, 1.0], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const orbitY = interpolate(local, [0, 150, 300], [-5, 3, -2], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const orbitZ = interpolate(local, [0, 150, 300], [-1.5, 1.2, -0.6], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const focus = interpolate(local, [180, 250, 330], [0.1, 0.75, 0.18], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });

  return (
    <div
      style={{
        position: "relative",
        width: 740,
        height: 1500,
        transform: `perspective(1800px) rotateX(${orbitY}deg) rotateY(${orbitY / 2}deg) rotateZ(${orbitZ}deg) scale(${pushIn}) translateY(${(1 - intro) * 180}px)`,
        opacity: intro,
        transformOrigin: "50% 50%",
      }}
    >
      <div
        style={{
          position: "absolute",
          inset: 0,
          borderRadius: 82,
          backgroundColor: COLORS.card,
          border: `3px solid ${COLORS.elevated}`,
          boxShadow: "0 40px 120px rgba(0,0,0,0.72), inset 0 0 0 2px rgba(255,255,255,0.03)",
          padding: 20,
          boxSizing: "border-box",
        }}
      >
        <div
          style={{
            position: "relative",
            width: "100%",
            height: "100%",
            overflow: "hidden",
            borderRadius: 64,
            backgroundColor: "#0B0B0D",
          }}
        >
          {useLivePhoneCapture ? (
            <Video
              src={staticFile("captures/pocketagent-demo.mp4")}
              muted
              style={{
                width: "100%",
                height: "100%",
                objectFit: "cover",
                objectPosition: "center",
                filter: `blur(${(1 - focus) * 1.8}px)`,
              }}
            />
          ) : (
            <Img
              src={staticFile("captures/phone-demo-start.png")}
              style={{
                width: "100%",
                height: "100%",
                objectFit: "cover",
                objectPosition: "center top",
                filter: `blur(${(1 - focus) * 1.8}px)`,
              }}
            />
          )}
          <div
            style={{
              position: "absolute",
              left: 22,
              right: 22,
              top: 22,
              height: 160,
              borderRadius: 28,
              background:
                "linear-gradient(115deg, rgba(255,255,255,0.14), rgba(255,255,255,0.01) 35%, rgba(255,255,255,0) 70%)",
              opacity: 0.6,
              pointerEvents: "none",
            }}
          />
          {!useLivePhoneCapture ? (
            <div
              style={{
                position: "absolute",
                left: 28,
                bottom: 34,
                padding: "12px 16px",
                borderRadius: 999,
                backgroundColor: "rgba(0,0,0,0.7)",
                color: COLORS.text,
                fontFamily: FONTS.mono,
                fontSize: 17,
                letterSpacing: "0.08em",
              }}
            >
              UI SHOWCASE · VERIFIED SCREEN
            </div>
          ) : null}
        </div>
      </div>
      <div
        style={{
          position: "absolute",
          right: -18,
          top: 340,
          width: 16,
          height: 100,
          borderRadius: 8,
          backgroundColor: COLORS.elevated,
        }}
      />
      <div
        style={{
          position: "absolute",
          left: -18,
          top: 240,
          width: 16,
          height: 140,
          borderRadius: 8,
          backgroundColor: COLORS.elevated,
        }}
      />
    </div>
  );
};
