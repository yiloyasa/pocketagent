import React from "react";
import {Easing, interpolate} from "remotion";
import {COLORS, FONTS} from "../styles/colors";

type FeaturePillarsProps = {
  frame: number;
  startFrame: number;
  features: readonly string[];
  values: readonly number[];
};

const accents = [COLORS.blue, COLORS.green, COLORS.purple, COLORS.blue, COLORS.green];

export const FeaturePillars: React.FC<FeaturePillarsProps> = ({
  frame,
  startFrame,
  features,
  values,
}) => {
  return (
    <div
      style={{
        display: "flex",
        alignItems: "flex-end",
        justifyContent: "space-between",
        width: 920,
        height: 920,
        padding: "0 20px 12px",
        boxSizing: "border-box",
        borderBottom: `1px solid ${COLORS.elevated}`,
      }}
    >
      {features.map((feature, index) => {
        const revealStart = startFrame + index * 12;
        const targetHeight = values[index] * 720;
        const height = interpolate(
          frame,
          [revealStart, revealStart + 22, revealStart + 44],
          [0, targetHeight, targetHeight],
          {
            extrapolateLeft: "clamp",
            extrapolateRight: "clamp",
            easing: Easing.bezier(0.16, 1, 0.3, 1),
          },
        );
        const labelOpacity = interpolate(
          frame,
          [revealStart + 12, revealStart + 30],
          [0, 1],
          {extrapolateLeft: "clamp", extrapolateRight: "clamp"},
        );
        return (
          <div
            key={feature}
            style={{
              width: 142,
              height: 820,
              display: "flex",
              flexDirection: "column",
              justifyContent: "flex-end",
              alignItems: "center",
              gap: 24,
            }}
          >
            <div
              style={{
                width: 92,
                height,
                minHeight: 1,
                borderRadius: "20px 20px 8px 8px",
                backgroundColor: accents[index],
                boxShadow: `0 0 48px ${accents[index]}55`,
              }}
            />
            <div
              style={{
                color: COLORS.text,
                fontFamily: FONTS.mono,
                fontSize: 18,
                fontWeight: 600,
                letterSpacing: "0.08em",
                opacity: labelOpacity,
              }}
            >
              {feature.toUpperCase()}
            </div>
          </div>
        );
      })}
    </div>
  );
};
