import { NextResponse } from "next/server";
import { clerkMiddleware, createRouteMatcher } from "@clerk/nextjs/server";

const isPublicRoute = createRouteMatcher([
  "/",
  "/join(.*)",
  "/create-org(.*)",
]);

export default clerkMiddleware(async (auth, req) => {
  if (isPublicRoute(req)) {
    return NextResponse.next();
  }

  const { userId } = await auth();
  if (userId) {
    return NextResponse.next();
  }

  if (req.nextUrl.pathname.startsWith("/api") || req.nextUrl.pathname.startsWith("/trpc")) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const landingURL = new URL("/", req.url);
  landingURL.searchParams.set("auth", "required");
  return NextResponse.redirect(landingURL);
});

export const config = {
  matcher: [
    // Skip Next.js internals and all static files, unless found in search params
    '/((?!_next|[^?]*\\.(?:html?|css|js(?!on)|jpe?g|webp|png|gif|svg|ttf|woff2?|ico|csv|docx?|xlsx?|zip|webmanifest)).*)',
    // Always run for API routes
    '/(api|trpc)(.*)',
  ],
};
