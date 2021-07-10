/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React from "react";

import { ViewProps } from "./objbrowse";
import { useFetchJSON } from "./hooks";

import "./asmview.css";

type json = { Insts: inst[], LastPC: string }
type inst = { PC: string, Op: string, Args: string[], Control?: control }
type control = { Type: number, Conditional: boolean, TargetPC: string }

function AsmViewer(props: ViewProps) {
    const fetch = useFetchJSON(`/sym/${props.entity.id}/asm`)
    if (fetch.pending) {
        return fetch.pending;
    }
    const v: json = fetch.value;

    // Create the instruction rows.
    const rows: React.ReactElement[] = [];
    const basePC = BigInt("0x" + v.Insts[0].PC);
    for (let inst of v.Insts) {
        const pc = BigInt("0x" + inst.PC);
        const pcDelta = pc - basePC;

        rows.push(
            <tr>
                <td className="av-addr">0x{inst.PC}</td>
                <td className="av-addr">+0x{pcDelta.toString(16)}</td>
                <td className="av-inst">{inst.Op}</td>
                <td className="av-inst">{formatArgs(inst.Args)}</td>
            </tr>
        );
    }

    return <table className="av-table">{rows}</table>;
}

function formatArgs(args: string[]): React.ReactElement[] {
    // TODO: Link symbols. I probably need exact SymIDs to do this
    // reliably, rather than depending on the name in the text.
    return [<>{args.join(", ")}</>];
}

export const AsmView = { element: AsmViewer, id: "asm", label: "Assembly" };
