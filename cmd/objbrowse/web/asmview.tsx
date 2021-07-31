/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React from "react";

import { ViewProps, Entity, Selection } from "./objbrowse";
import { useFetchJSON } from "./hooks";
import { Ranges, Range } from "./ranges";

import "./asmview.css";

type json = { Insts: inst[], Refs: symRef[], LastPC: string }
type inst = { PC: string, Op: string, Args: string, Control?: control }
type symRef = { ID: number, Name: string, Addr: string }
type control = { Type: number, Conditional: boolean, TargetPC: string }

// TODO: Implement selecting an instruction or range of instructions.

// TODO: Display control flow. Showing all of the arrows is a little
// overwhelming. Maybe I should just show outgoing/incoming arrows for
// the selected (moused-over if no selection) instruction? Doing just
// mouse-over doesn't help for viewing long jumps. Maybe I always mark
// instructions that are the target of a jump, just not with the whole
// arrow.

// TODO: Display source lines for each instruction, including their
// inline info (on hover or something).

function AsmViewer(props: ViewProps) {
    // Fetch data.
    const fetch = useFetchJSON(`/sym/${props.value.entity.id}/asm`)
    if (fetch.pending) {
        return fetch.pending;
    }
    const v: json = fetch.value;

    // Parse PCs and compute lengths of all instructions.
    let pcs: bigint[] = [];
    let ranges: Range[] = [];
    for (let i = 0; i < v.Insts.length; i++) {
        pcs.push(BigInt("0x" + v.Insts[i].PC));
    }
    pcs.push(BigInt("0x" + v.LastPC));
    for (let i = 0; i < v.Insts.length; i++) {
        ranges.push({ start: pcs[i], end: pcs[i + 1] });
    }
    // Ranges will sort this by start, but that's okay because it's
    // already sorted by start.
    let rangeMap = new Ranges(ranges);

    // Create the instruction rows.
    const rows: React.ReactElement[] = [];
    for (let i = 0; i < v.Insts.length; i++) {
        const inst = v.Insts[i];
        const range = ranges[i];

        let className = "";
        if (props.value.ranges.anyIntersection(range)) {
            className = "ob-selected";
        }

        rows.push(
            <tr key={i} className={className} onClick={() => props.onSelectRange(new Ranges(range))}>
                <td className="av-addr">0x{inst.PC}</td>
                <td className="av-addr">+0x{(pcs[i] - pcs[0]).toString(16)}</td>
                <td className="av-inst">{inst.Op}</td>
                <td className="av-inst">{formatArgs(inst.Args, v.Refs, rangeMap, props.value.entity.id, props.onSelect)}</td>
            </tr >
        );
    }

    return <table className="av-table"><tbody>{rows}</tbody></table>;
}

function formatArgs(args: string, symRefs: symRef[], ranges: Ranges, selfID: number, onSelect: (sel: Selection) => void): React.ReactElement {
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
            const entity: Entity = { type: "sym", id: sym.ID };
            const addr = BigInt("0x" + sym.Addr) + BigInt(offset);
            // If this is a reference to our own symbol, find the range
            // containing offset so we can select the whole instruction.
            const selfRange = sym.ID == selfID ? ranges.find(addr) : null;
            const range = selfRange || { start: addr, end: addr + BigInt(1) };
            onSelect({ entity, ranges: new Ranges([range]) });
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
