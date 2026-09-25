"use client";

import { ErrorState } from "@/components/states";
import { PageHeader } from "@/components/layout";
import { Button } from "@/components/ui/button";

// Unexpected rendering failure inside the product area: the shell stays, and
// the user gets a safe message with a retry (no stack traces).
export default function ProductError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <><PageHeader title="Page unavailable" />
    <ErrorState
      title="This page could not be displayed"
      error={null}
      action={
        <Button variant="secondary" size="sm" onClick={reset}>
          Try again
        </Button>
      }
    />
    </>
  );
}
