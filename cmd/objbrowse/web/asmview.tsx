/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React, { useState, useRef, useMemo } from "react";

import { ViewProps, Entity, Selection } from "./objbrowse";
import { useFetchJSON } from "./hooks";
import { Ranges, Range } from "./ranges";
import * as History from "./history";

import "./asmview.css";

type json = { Insts: inst[], Refs: symRef[], LastPC: string }
type inst = { PC: string, Op: string, Args: string, Control?: control }
type symRef = { ID: number, Name: string, Addr: string }
type control = { Type: string, Conditional: boolean, TargetPC?: string }

// TODO: Display source lines for each instruction, including their
// inline info (on hover or something). If there's inlining, showing the
// innermost function name of each instruction may be particularly
// valuable.

function AsmViewer(props: ViewProps) {
    // Fetch data.
    const fetch = useFetchJSON(`/sym/${props.value.entity.id}/asm`)
    if (fetch.pending) {
        return fetch.pending;
    }
    const v: json = fetch.value;
    return <AsmViewer1 {...props} v={v} />;
}

interface AsmViewer1Props extends ViewProps {
    v: json;
}

function AsmViewer1(props: AsmViewer1Props) {
    console.log("render AsmViewer1");
    const v = props.v;

    // Parse PCs.
    let pcs = useMemo(() => {
        let pcs = v.Insts.map(inst => BigInt("0x" + inst.PC));
        pcs.push(BigInt("0x" + v.LastPC));
        return pcs;
    }, [v]);
    // Compute ranges for each instruction.
    let [ranges, rangeMap] = useMemo(() => {
        let ranges = pcs.map((pc, i) => ({ start: pc, end: pcs[i + 1], index: i }));
        return [ranges, new Ranges(ranges, "sorted")];
    }, [pcs]);
    // Invert control flow.
    let sources = useMemo(() => {
        let sources = new Array<number[]>(v.Insts.length);
        for (let [source, inst] of v.Insts.entries()) {
            if (inst.Control?.TargetPC !== undefined) {
                let target = BigInt("0x" + inst.Control.TargetPC);
                let i = rangeMap.find(target)?.index;
                if (i !== undefined) {
                    if (sources[i] === undefined) {
                        sources[i] = [];
                    }
                    sources[i].push(source);
                }
            }
        }
        return sources;
    }, [v, rangeMap]);

    // Create the instruction rows.
    let rows: React.ReactElement[] = [];
    // We use rowRefs to measure the DOM and create new DOM. This
    // circular dependency is tricky. If, while creating new DOM, we
    // find that we're missing refs, we set refsStale. Then, if a ref
    // gets updated, it will bump refGen to force a re-render.
    let rowRefs = useRef<(HTMLTableRowElement | undefined)[]>([]); // By instruction index.
    let refsStale = false;
    let [_, setRefGen] = useState(0);
    for (let i = 0; i < v.Insts.length; i++) {
        const inst = v.Insts[i];
        const range = ranges[i];

        let selected = props.value.ranges.anyIntersection(range);
        let className = selected ? "ob-selected" : "";

        let target;
        if (sources[i] !== undefined) {
            let sourceLine;
            let sourceMarks = [];
            if (selected) {
                let mr = rowRefs.current[i];
                if (mr === undefined) {
                    refsStale = true;
                } else {
                    let my = mr.getBoundingClientRect().y;
                    let min = my, max = my;
                    for (let j of sources[i]) {
                        let tr = rowRefs.current[j];
                        if (tr === undefined) {
                            refsStale = true;
                            continue;
                        }
                        let ty = tr.getBoundingClientRect().y;
                        if (ty < min) {
                            min = ty;
                        }
                        if (ty > max) {
                            max = ty;
                        }
                        // Add a mark for this source.
                        sourceMarks.push(<circle key={j} cx="0" cy={ty - my} r="5" />);
                    }
                    // Create a single line from the highest to lowest source.
                    sourceLine = <path d={`M0 ${min - my}V${max - my}`} stroke="black" strokeWidth="2px" />;
                }
            }

            target = <svg style={{ overflow: "visible", width: "16px", height: "1px" }}>
                <path d={`M${selected ? 0 : 8} 0H15`} stroke="black" strokeWidth="2px" />
                <path d="M16 0l-5 -3v6z" /> {/* Arrow head */}
                {sourceLine}
                {sourceMarks}
            </svg>
        }

        rows.push(
            <tr key={i} className={className} data-i={i} onClick={() => props.onSelectRange(new Ranges(range))}
                ref={elt => {
                    if (elt === null) {
                        rowRefs.current[i] = undefined;
                    } else {
                        if (rowRefs.current[i] === undefined && refsStale) {
                            // Force a re-render.
                            setRefGen(gen => gen + 1);
                        }
                        rowRefs.current[i] = elt;
                    }
                }}>
                <td className="av-addr">0x{inst.PC}</td>
                <td className="av-addr">+0x{(pcs[i] - pcs[0]).toString(16)}</td>
                <td>{target}</td>
                <td className="av-inst">{inst.Op}</td>
                <td className="av-inst">{formatArgs(inst.Args, v.Refs, rangeMap, props.value.entity, props.onSelect)}</td>
            </tr >
        );

        // If this is an unconditional flow exit, leave a blank.
        if (inst.Control?.Conditional === false && (inst.Control.Type == "Jump" || inst.Control.Type == "Ret" || inst.Control.Type == "Exit")) {
            rows.push(<tr key={i + "x"} className="av-unconditional" />)
        }
    }

    return <table className="av-table"><tbody>{rows}</tbody></table>;
}

function formatArgs(args: string, symRefs: symRef[], ranges: Ranges, self: Entity, onSelect: (sel: Selection, push?: History.PushKind) => void): React.ReactElement {
    if (!args.includes("\u00ab")) {
        // No symbolic references.
        return <>{args}</>;
    }

    let parts: (React.ReactElement | string)[] = [];
    let re = /\u00ab(\d+)\+([0-9a-fA-F]+)\u00bb/g;
    let start = 0;
    let m;
    while ((m = re.exec(args)) !== null) {
        if (m.index > 0) {
            parts.push(args.substring(start, m.index));
        }
        const sym = symRefs[parseInt(m[1], 10)];
        const offset = parseInt(m[2], 16);
        let text = sym.Name;
        if (offset != 0) {
            text += `+0x${offset.toString(16)}`;
        }
        parts.push(<a key={start} href="#" onClick={(ev) => {
            const addr = BigInt("0x" + sym.Addr) + BigInt(offset);
            let range = { start: addr, end: addr + BigInt(1) };
            let entity: Entity;
            if (sym.ID == self.id) {
                // This is a reference to our own symbol. Find the range
                // containing offset so we can select the whole
                // instruction.
                //
                // Use the self object itself (rather than making a new
                // entity) to reduce re-rendering.
                entity = self;
                range = ranges.find(addr) || range;
            } else {
                entity = { type: "sym", id: sym.ID };
            }
            onSelect({ entity, ranges: new Ranges(range) }, "push");
            ev.preventDefault();
            // Prevent the row click, which will try to select the row.
            ev.stopPropagation();
        }}>{text}</a>);
        start = re.lastIndex;
    }
    // Remainder of argument.
    parts.push(args.substring(start));
    return <>{parts}</>;
}

export const AsmView = { element: AsmViewer, id: "asm", label: "Assembly" };
