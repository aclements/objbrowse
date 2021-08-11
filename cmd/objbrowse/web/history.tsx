/** @license ObjBrowse
 * Copyright 2021 The Go Authors. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

import React, { useCallback, useEffect } from "react";

//const debugLog = console.log
const debugLog = (...rest: any[]) => { }

/**
 * pendingState is the state to be committed. Usually, this should match
 * window.history.state, except when a commit is queued.
 */
let pendingState: { [key: string]: any } = {};
let poppers = new Map<string, (val: any) => void>();

/**
 * Key identifies a component of history state uniquely within an
 * application. It should not be constructed directly by callers.
 */
export class Key {
    _key: string;
    constructor(key: string) {
        this._key = key;
    }

    sub(name: string): Key {
        return new Key(this._key + "/" + name);
    }
}

let registered = false;
/**
 * newHistoryRoot returns a root history Key. Because history state is
 * window-global, there must only be one root history Key per window.
 */
export function newHistoryRoot(): Key {
    if (registered) {
        throw "History root already created";
    }
    registered = true;

    const popstate = () => {
        if (window.history.state === null) {
            debugLog("pop to initial");
            // Reset everything to initial state.
            poppers.forEach(popper => popper(null));
        } else if (typeof window.history.state == "object") {
            poppers.forEach((popper, key) => {
                if (key in window.history.state) {
                    popper(window.history.state[key]);
                } else {
                    popper(null);
                }
            });
        }
    };

    window.addEventListener('popstate', popstate);

    return new Key("");
}

export type PushKind = "replace" | "push";

let commitQueued: null | PushKind = null;
function queueCommit(kind?: PushKind) {
    if (kind === undefined) {
        kind = "replace";
    }
    if (commitQueued === null) {
        commitQueued = kind;
        setTimeout(commitState, 0);
    } else if (commitQueued === "replace" && kind === "push") {
        // If any changes are a push, the whole thing is a push.
        commitQueued = "push";
    }
}
function commitState() {
    if (commitQueued === "push") {
        debugLog("pushState", pendingState);
        window.history.pushState(pendingState, "");
    } else {
        debugLog("replaceState", pendingState);
        window.history.replaceState(pendingState, "");
    }
    commitQueued = null;
}

/**
 * useState is like React.useState, but additionally synchronizes the
 * state to the window history.
 *
 * @param key The history key under which to store this state. This must
 * be unique across all current components.
 * @param initial Initial value of the state. If the history already has
 * a valid value for this key, this will use that value instead.
 * @param encode Encode the state into a serializable history state.
 * @param decode Decode the history state into a T. This should also
 * validate the state and return undefined for invalid states.
 * @returns [value, setValue] The current value of this state, and
 * function to set its value. setValue takes an optional second argument
 * that can be "push" or "replace" indicating whether this change should
 * push a new entry on the history stack, or replace the current top of
 * stack. If updates to any state during a refresh are "push", then the
 * new history state will be pushed.
 */
export function useState<T>(key: Key, initial: T, encode: (v: T) => any, decode: (v: any) => (T | undefined)): [T, (v: T, kind?: PushKind) => void] {
    return useReducer(key,
        useCallback((old: T, action: T) => action, []),
        initial,
        encode, decode);
}

/**
 * useReducer is like useState, but it takes a state reducer function
 * and returns a dispatch function, like React.useReducer.
 */
export function useReducer<T, A>(key: Key, reducer: (old: T, action: A) => T, initial: T, encode: (v: T) => any, decode: (v: any) => (T | undefined)): [T, (action: A, kind?: PushKind) => void] {
    // Create the React state underlying this history state.
    const [value, setValue] = React.useState(() => {
        if (window.history.state !== null && key._key in window.history.state) {
            // Use the state decoded from the history.
            let encoded = window.history.state[key._key];
            try {
                let n = decode(encoded);
                if (n !== undefined) {
                    debugLog("decoded initial state", key._key, n);
                    //pendingState.set(key._key, encoded);
                    pendingState[key._key] = encoded;
                    return n;
                }
            } catch (e) {
                // Assume state was malformed and fall back to initial value.
                console.error("decoding history state for " + key._key, e, e.stack);
            }
        }
        return initial;
    });

    // Create the state-updater function that updates the underlying
    // React state and also pushes the history state.
    const dispatch = useCallback((action: A, kind?: PushKind) => {
        setValue(old => {
            let v = reducer(old, action);
            if (Object.is(old, v)) {
                debugLog("dispatch no change", old);
                return old;
            }
            debugLog("dispatch", key._key, v);
            // Store the pending state.
            pendingState[key._key] = encode(v);
            // Queue a push.
            queueCommit(kind);
            return v;
        });
    }, [reducer]);

    // Register with the global popState dispatcher.
    //
    // This technically depends on initial and decode, but we assume
    // those are fixed after the first call. (Otherwise this requires a
    // lot of cumbersome memoization on the caller's part.)
    useEffect(() => {
        poppers.set(key._key, val => {
            debugLog("popState", key._key, val);
            pendingState[key._key] = val;
            if (val !== null) {
                try {
                    let n = decode(val);
                    if (n !== undefined) {
                        setValue(n);
                        return;
                    }
                } catch (e) {
                    console.error("decoding history state for " + key._key, e, e.stack);
                }
            }
            setValue(initial);
        });

        // Prepare to deregister state and dispatcher.
        return () => {
            debugLog("deregister", key._key);
            poppers.delete(key._key);
            delete pendingState[key._key];
        };
    }, [setValue]);

    return [value, dispatch];
}
