import React from "react";
import {interpolate, useCurrentFrame} from "remotion";
import {COLORS, FONTS} from "../styles/colors";

type EventTraceProps = {
  compact?: boolean;
  accent?: string;
};

export const EventTrace: React.FC<EventTraceProps> = ({
  compact = false,
  accent = COLORS.blue,
}) => {
  const frame = useCurrentFrame();
  const draw = interpolate(frame, [0, compact ? 38 : 76], [0, 1], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const pulse = interpolate(frame, [0, 90, 180], [0.7, 1, 0.75], {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });
  const width = compact ? 760 : 920;
  const height = compact ? 180 : 280;
  const path = compact
    ? "M24 138 C160 32 250 145 356 76 S540 42 710 116"
    : "M18 218 C180 24 316 250 470 92 S690 42 902 188";

  return (
    <div style={{position: "relative", width, height}}>
      <svg
        viewBox={`0 0 ${width} ${height}`}
        width={width}
        height={height}
        style={{overflow: "visible"}}
      >
        <path
          d={path}
          fill="none"
          stroke={accent}
          strokeOpacity={0.16}
          strokeWidth={compact ? 20 : 28}
          strokeLinecap="round"
          style={{filter: "blur(12px)"}}
        />
        <path
          d={path}
          fill="none"
          stroke={accent}
          strokeOpacity={0.9}
          strokeWidth={compact ? 3 : 4}
          strokeLinecap="round"
          pathLength={1}
          strokeDasharray={1}
          strokeDashoffset={1 - draw}
        />
        {[compact ? [160, 77] : [184, 58], compact ? [356, 76] : [470, 92], compact ? [540, 42] : [690, 42]].map(
          ([cx, cy], index) => (
            <circle
              key={index}
              cx={cx}
              cy={cy}
              r={compact ? 7 : 9}
              fill={accent}
              opacity={draw * pulse}
              style={{filter: "drop-shadow(0 0 12px rgba(74,157,255,0.8))"}}
            />
          ),
        )}
      </svg>
      {!compact ? (
        <div
          style={{
            position: "absolute",
            right: 12,
            bottom: 18,
            color: COLORS.muted,
            fontFamily: FONTS.mono,
            fontSize: 20,
            letterSpacing: "0.14em",
            opacity: draw,
          }}
        >
          STREAM / APPROVAL / RESULT
        </div>
      ) : null}
    </div>
  );
};
