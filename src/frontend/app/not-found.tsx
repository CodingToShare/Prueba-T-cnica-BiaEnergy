import Link from "next/link";

import { NotFoundState } from "@/components/states";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <main className="grid min-h-dvh place-items-center px-4">
      <div className="w-full max-w-md">
        <NotFoundState
          headingLevel={1}
          title="Page not found"
          message="The page you are looking for does not exist."
          action={
            <Button asChild size="sm">
              <Link href="/">Go to Bia Energy</Link>
            </Button>
          }
        />
      </div>
    </main>
  );
}
