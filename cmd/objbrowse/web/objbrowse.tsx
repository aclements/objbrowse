/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React, { useState } from "react";
import "bootstrap/dist/css/bootstrap.min.css";
import "./objbrowse.css";

import { useFetchJSON } from "./hooks";

type Entity = { type: "sym", id: number }
type MaybeEntity = null | Entity

export interface ViewProps { entity: Entity }
export interface View { element: (props: ViewProps) => JSX.Element, name: string }

export interface AppProps { views: View[] }

export function App(props: AppProps) {
    // TODO: Sync current entity (and selected range in that entity) to the URL
    // history.
    const [selected, setSelected] = useState<MaybeEntity>(null);

    // TODO: This just loops over all the views, but I need to present
    // just the views that support the current entity and panel them in
    // some way (which will clean up the styling on the second column).
    return (
        <div className="ob-root">
            <div className="container-fluid">
                <div className="row flex-xl-nowrap">
                    <div className="col-2 p-0">
                        <SymPanel value={selected} onSelect={setSelected} />
                    </div>
                    <div className="col-10" style={{ height: "100vh", overflow: "scroll" }}>
                        {selected !== null && props.views.map((View) =>
                            <View.element key={View.name} entity={selected}></View.element>)
                        }
                    </div>
                </div>
            </div>
        </div>
    );
}

interface SymPanelProps { value: MaybeEntity, onSelect: (ent: MaybeEntity) => void }

function SymPanel(props: SymPanelProps) {
    const [filterStr, setFilterStr] = useState("");
    const [filter, setFilter] = useState(() => new RegExp(filterStr));
    const [isError, setError] = useState(false);

    // TODO: Updating the filter can be rather slow with a large symbol list.
    // Consider updating after a short delay, or, ideally, reduce the cost of
    // the symbol list (e.g., with a lazy table).

    function handleChange(val: string) {
        setFilterStr(val);
        try {
            setFilter(new RegExp(val));
            setError(false);
        } catch (e) {
            setError(true);
        }
    }

    // TODO: Filter by address.
    // TODO: Filter by symbol type?

    return (<div className="ob-sympanel bg-light">
        <div className="m-3">
            <input value={filterStr} onChange={(ev) => handleChange(ev.target.value)} placeholder="Symbol regexp" className={isError ? "bg-warning" : ""}></input>
        </div>
        {
            // Don't add left or right margin. The SymList rows add their own.
        }
        <div className="my-3">
            <SymList filter={filter} value={props.value} onSelect={props.onSelect} />
        </div>
    </div >);
}

interface SymListProps { filter: RegExp, value: MaybeEntity, onSelect: (ent: MaybeEntity) => void }

function SymList(props: SymListProps) {
    const symsJSON = useFetchJSON("/syms");
    if (symsJSON.pending) {
        return <div className="mx-3">{symsJSON.pending}</div>;
    }
    const syms: string[] = symsJSON.value;
    // TODO: Indicate symbol type? Perhaps to the left of the symbol?
    //
    // TODO: Display columns with much more symbol information? I would probably
    // have to limit the symbol name length and make the SymPanel expandable.
    return (
        <div className="ob-symlist-outer">
            <ul className="ob-symlist list-unstyled text-nowrap">
                {syms.map((data, id) => {
                    if (props.filter.test(data)) {
                        let cls = "";
                        if (props.value !== null && props.value.type == "sym" && props.value.id == id) {
                            cls = "ob-symlist-selected";
                        }
                        return <li key={id} onClick={() => props.onSelect({ type: "sym", id: id })} className={cls}> {data}</li>;
                    }
                })}
            </ul >
        </div>
    );
}
